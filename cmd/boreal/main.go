package main

import (
	"log"
	"os"
	"runtime/debug"
	"time"

	"gioui.org/app"
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
	app.Main()
}
