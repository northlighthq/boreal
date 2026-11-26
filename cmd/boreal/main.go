package main

import (
	"log"
	"os"
	"runtime/debug"
	"time"

	"gioui.org/app"

	"github.com/northlighthq/boreal/internal/platform/instance"
)

func main() {
	args := os.Args[1:]

	if code, ok := subcommand(args); ok {
		os.Exit(code)
	}

	if handoff(args) {
		return
	}

	debug.SetGCPercent(50)
	go func() {
		time.Sleep(4 * time.Second)
		debug.FreeOSMemory()
	}()
	go func() {
		if err := run(args); err != nil {
			log.Fatal(err)
		}

		os.Exit(0)
	}()
	for e := range app.Events {
		if u, ok := e.(app.URLEvent); ok && u.URL != nil {
			instance.Deliver([]string{u.URL.String()})
		}
	}
}
