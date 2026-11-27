package main

import (
	"log"
	"os"
	"runtime/debug"
	"time"

	"gioui.org/app"

	"github.com/northlighthq/boreal/internal/core/launch"
	boreal "github.com/northlighthq/boreal/internal/desktop"
	borealmcp "github.com/northlighthq/boreal/internal/mcp"
	"github.com/northlighthq/boreal/internal/platform/instance"
)

func main() {
	args := os.Args[1:]

	if len(args) == 1 && args[0] == borealmcp.Command {
		os.Exit(borealmcp.Main())
	}

	if _, ok := launch.Parse(args); ok && instance.Send(args) {
		return
	}

	debug.SetGCPercent(50)
	go func() {
		time.Sleep(4 * time.Second)
		debug.FreeOSMemory()
	}()
	go func() {
		if err := boreal.Run(args); err != nil {
			log.Fatal(err)
		}

		os.Exit(0)
	}()
	for event := range app.Events {
		if opened, ok := event.(app.URLEvent); ok && opened.URL != nil {
			instance.Deliver([]string{opened.URL.String()})
		}
	}
}
