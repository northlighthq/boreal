package main

import (
	"log"
	"os"
	"runtime/debug"
	"time"

	"gioui.org/app"

	boreal "github.com/northlighthq/boreal/internal/app"
)

func main() {
	debug.SetGCPercent(50)
	go func() {
		time.Sleep(4 * time.Second)
		debug.FreeOSMemory()
	}()
	go func() {
		if err := boreal.Run(); err != nil {
			log.Fatal(err)
		}

		os.Exit(0)
	}()
	app.Main()
}
