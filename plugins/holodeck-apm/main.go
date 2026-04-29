// Package main is the holodeck APM plugin entry point.
package main

import (
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	holodeck "github.com/gulducat/autoscaler-holodeck/plugins/holodeck-apm/plugin"
)

func main() {
	plugins.Serve(factory)
}

func factory(log hclog.Logger) interface{} {
	return holodeck.New(log)
}
