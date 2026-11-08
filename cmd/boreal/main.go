package main

import (
	"log"
	"os"
	"runtime/debug"
	"time"

	"gioui.org/app"

	"github.com/northlighthq/boreal/internal/core/launch"
	boreal "github.com/northlighthq/boreal/internal/desktop"
	"github.com/northlighthq/boreal/internal/platform/instance"
)

func main() {
	args := os.Args[1:]

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
	app.Main()
}
