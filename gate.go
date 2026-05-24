package main

import (
	"github.com/minekube/gate-plugin-template/plugins/whitelist"
	"go.minekube.com/gate/cmd/gate"
	"go.minekube.com/gate/pkg/edition/java/proxy"
)

func main() {
	proxy.Plugins = append(proxy.Plugins,
		whitelist.Plugin,
	)

	gate.Execute()
}
