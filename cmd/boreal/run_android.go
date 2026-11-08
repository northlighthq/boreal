package main

import "github.com/northlighthq/boreal/internal/mobile"

func subcommand(args []string) (int, bool) {
	return 0, false
}

func handoff(args []string) bool {
	return false
}

func run(args []string) error {
	return mobile.Run()
}
