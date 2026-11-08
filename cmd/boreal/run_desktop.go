//go:build !android

package main

import (
	"os"

	"github.com/northlighthq/boreal/internal/core/launch"
	"github.com/northlighthq/boreal/internal/desktop"
	borealmcp "github.com/northlighthq/boreal/internal/mcp"
	"github.com/northlighthq/boreal/internal/mobile"
	"github.com/northlighthq/boreal/internal/platform/instance"
)

func subcommand(args []string) (int, bool) {
	if len(args) == 1 && args[0] == borealmcp.Command {
		return borealmcp.Main(), true
	}

	return 0, false
}

func handoff(args []string) bool {
	_, ok := launch.Parse(args)
	return ok && instance.Send(args)
}

func run(args []string) error {
	if os.Getenv("BOREAL_MOBILE") != "" {
		return mobile.Run()
	}

	return desktop.Run(args)
}
