// Package main is the nodesim target plugin entry point.
package main

import (
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	nodesim "github.com/gulducat/autoscaler-holodeck/plugins/nodesim-target/plugin"
)

func main() {
	plugins.Serve(factory)
}

func factory(log hclog.Logger) interface{} {
	return nodesim.New(log)
}
