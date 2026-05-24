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
			w.names[strings.ToLower(name)] = true
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
	return w.names[strings.ToLower(name)]
}

func (w *whitelist) add(name string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	lower := strings.ToLower(name)
	if w.names[lower] {
		return false
	}
	w.names[lower] = true
	return true
}

func (w *whitelist) remove(name string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	lower := strings.ToLower(name)
	if !w.names[lower] {
		return false
	}
	delete(w.names, lower)
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

func msg(content string, clr color.Color) c.Component {
	return &c.Text{Content: content, S: c.Style{Color: clr}}
}

func msgBold(content string, clr color.Color) c.Component {
	return &c.Text{Content: content, S: c.Style{Color: clr, Bold: true}}
}

func buildMsg(parts ...c.Component) c.Component {
	return &c.Text{Extra: append(
		[]c.Component{msg("[WhitelistGate] ", color.DarkGreen)},
		parts...,
	)}
}

func whitelistCommand() brigodier.LiteralNodeBuilder {
	return brigodier.Literal("whitelistgate").
		Then(brigodier.Literal("add").
			Then(brigodier.Argument("name", brigodier.String).
				Executes(command.Command(func(ctx *command.Context) error {
					name := ctx.String("name")
					if wl.add(name) {
						_ = wl.save()
						return ctx.Source.SendMessage(buildMsg(
							msgBold(name, color.Yellow),
							msg(" has been added to the whitelist", color.Green),
						))
					}
					return ctx.Source.SendMessage(buildMsg(
						msgBold(name, color.Yellow),
						msg(" is already whitelisted", color.Red),
					))
				})))).
		Then(brigodier.Literal("remove").
			Then(brigodier.Argument("name", brigodier.String).
				Executes(command.Command(func(ctx *command.Context) error {
					name := ctx.String("name")
					if wl.remove(name) {
						_ = wl.save()
						return ctx.Source.SendMessage(buildMsg(
							msgBold(name, color.Yellow),
							msg(" has been removed from the whitelist", color.Green),
						))
					}
					return ctx.Source.SendMessage(buildMsg(
						msgBold(name, color.Yellow),
						msg(" is not on the whitelist", color.Red),
					))
				})))).
		Then(brigodier.Literal("list").
			Executes(command.Command(func(ctx *command.Context) error {
				names := wl.namesList()
				if len(names) == 0 {
					return ctx.Source.SendMessage(buildMsg(
						msg("Whitelist is empty", color.Yellow),
					))
				}
				return ctx.Source.SendMessage(buildMsg(
					msg("Whitelisted players (", color.Green),
					msgBold(fmt.Sprintf("%d", len(names)), color.Yellow),
					msg("): ", color.Green),
					msgBold(strings.Join(names, ", "), color.White),
				))
			}))).
		Then(brigodier.Literal("reload").
			Executes(command.Command(func(ctx *command.Context) error {
				if err := wl.load(); err != nil {
					return ctx.Source.SendMessage(buildMsg(
						msg("Failed to reload: ", color.Red),
						msg(err.Error(), color.Red),
					))
				}
				return ctx.Source.SendMessage(buildMsg(
					msg("Whitelist reloaded. ", color.Green),
					msgBold(fmt.Sprintf("%d", len(wl.names)), color.Yellow),
					msg(" players whitelisted", color.Green),
				))
			})))
}
