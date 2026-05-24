package whitelist

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	"github.com/robinbraemer/event"
	"go.minekube.com/brigodier"
	"go.minekube.com/common/minecraft/color"
	c "go.minekube.com/common/minecraft/component"
	"go.minekube.com/gate/pkg/command"
	"go.minekube.com/gate/pkg/edition/java/proxy"
)

const whitelistFile = "whitelist.txt"

type whitelist struct {
	mu    sync.RWMutex
	names map[string]bool
}

var wl = &whitelist{names: make(map[string]bool)}

var Plugin = proxy.Plugin{
	Name: "Whitelist",
	Init: func(ctx context.Context, p *proxy.Proxy) error {
		log := logr.FromContextOrDiscard(ctx)

		if err := wl.load(); err != nil {
			log.Error(err, "Failed to load whitelist")
		}
		log.Info("Loaded whitelist", "count", len(wl.names))

		event.Subscribe(p.Event(), 0, onLogin)
		p.Command().Register(whitelistCommand())

		return nil
	},
}

func (w *whitelist) load() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	f, err := os.Open(whitelistFile)
	if err != nil {
		if os.IsNotExist(err) {
			w.names = make(map[string]bool)
			return nil
		}
		return err
	}
	defer f.Close()

	w.names = make(map[string]bool)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		name := strings.TrimSpace(scanner.Text())
		if name != "" {
			w.names[name] = true
		}
	}
	return scanner.Err()
}

func (w *whitelist) save() error {
	w.mu.RLock()
	defer w.mu.RUnlock()

	f, err := os.Create(whitelistFile)
	if err != nil {
		return err
	}
	defer f.Close()

	for name := range w.names {
		if _, err := f.WriteString(name + "\n"); err != nil {
			return err
		}
	}
	return nil
}

func (w *whitelist) contains(name string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.names[name]
}

func (w *whitelist) add(name string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.names[name] {
		return false
	}
	w.names[name] = true
	return true
}

func (w *whitelist) remove(name string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.names[name] {
		return false
	}
	delete(w.names, name)
	return true
}

func (w *whitelist) namesList() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	names := make([]string, 0, len(w.names))
	for name := range w.names {
		names = append(names, name)
	}
	return names
}

func onLogin(e *proxy.LoginEvent) {
	if !wl.contains(e.Player().Username()) {
		e.Deny(&c.Text{
			Content: "You are not whitelisted on this server!",
			S:       c.Style{Color: color.Red},
		})
	}
}

func whitelistCommand() brigodier.LiteralNodeBuilder {
	return brigodier.Literal("whitelistgate").
		Requires(command.Requires(func(c *command.RequiresContext) bool {
			player, ok := c.Source.(proxy.Player)
			if !ok {
				return true
			}
			return player.HasPermission("whitelist.admin") || player.HasPermission("minecraft.command.op") || player.HasPermission("*")
		})).
		Then(brigodier.Literal("add").
			Then(brigodier.Argument("name", brigodier.String).
				Executes(command.Command(func(ctx *command.Context) error {
					name := ctx.String("name")
					if wl.add(name) {
						_ = wl.save()
						return ctx.Source.SendMessage(&c.Text{
							Content: "Added " + name + " to whitelist",
							S:       c.Style{Color: color.Green},
						})
					}
					return ctx.Source.SendMessage(&c.Text{
						Content: name + " is already whitelisted",
						S:       c.Style{Color: color.Red},
					})
				})))).
		Then(brigodier.Literal("remove").
			Then(brigodier.Argument("name", brigodier.String).
				Executes(command.Command(func(ctx *command.Context) error {
					name := ctx.String("name")
					if wl.remove(name) {
						_ = wl.save()
						return ctx.Source.SendMessage(&c.Text{
							Content: "Removed " + name + " from whitelist",
							S:       c.Style{Color: color.Green},
						})
					}
					return ctx.Source.SendMessage(&c.Text{
						Content: name + " is not on the whitelist",
						S:       c.Style{Color: color.Red},
					})
				})))).
		Then(brigodier.Literal("list").
			Executes(command.Command(func(ctx *command.Context) error {
				names := wl.namesList()
				if len(names) == 0 {
					return ctx.Source.SendMessage(&c.Text{
						Content: "Whitelist is empty",
						S:       c.Style{Color: color.Yellow},
					})
				}
				return ctx.Source.SendMessage(&c.Text{
					Content: "Whitelisted: " + strings.Join(names, ", "),
					S:       c.Style{Color: color.Green},
				})
			}))).
		Then(brigodier.Literal("reload").
			Executes(command.Command(func(ctx *command.Context) error {
				if err := wl.load(); err != nil {
					return ctx.Source.SendMessage(&c.Text{
						Content: "Failed to reload whitelist: " + err.Error(),
						S:       c.Style{Color: color.Red},
					})
				}
				return ctx.Source.SendMessage(&c.Text{
					Content: fmt.Sprintf("Whitelist reloaded (%d players)", len(wl.names)),
					S:       c.Style{Color: color.Green},
				})
			})))
}
