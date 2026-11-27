//go:build darwin

package main

import (
	"fmt"
	"os"
	"time"

	"gioui.org/app"

	"github.com/northlighthq/boreal/internal/core/launch"
	"github.com/northlighthq/boreal/internal/platform/hello"
	"github.com/northlighthq/boreal/internal/platform/instance"
	"github.com/northlighthq/boreal/internal/platform/sessionlock"
	"github.com/northlighthq/boreal/internal/platform/urlhandler"
)

const received = "/tmp/boreal-url-received"

func checks() int {
	failed := 0
	report := func(name string, ok bool, detail any) {
		status := "ok"

		if !ok {
			status, failed = "FAIL", failed+1
		}

		fmt.Printf("%-28s %-4s %v\n", name, status, detail)
	}

	report("sessionlock.Locked", !sessionlock.Locked(), "unlocked runner session")
	report("hello.Available", true, hello.Available())
	report("hello.Verify without Touch ID", hello.Verify(0, "Unlock Boreal") != nil, hello.Verify(0, "Unlock Boreal"))

	err := urlhandler.Set(true)
	report("urlhandler.Set(true)", err == nil, err)
	report("urlhandler.Registered", urlhandler.Registered(), urlhandler.Registered())

	return failed
}

func release() int {
	err := urlhandler.Set(false)
	registered := urlhandler.Registered()
	fmt.Printf("%-28s %-4s %v / registered=%v\n", "urlhandler.Set(false)", map[bool]string{true: "ok", false: "FAIL"}[err == nil && !registered], err, registered)

	if err != nil || registered {
		return 1
	}

	return 0
}

func serve() {
	go func() {
		stop, err := instance.Listen(func(args []string) {
			request, ok := launch.Parse(args)
			line := fmt.Sprintf("args=%q parsed=%v kind=%s user=%s host=%s port=%d dir=%s\n", args, ok, request.Kind, request.Host.Username, request.Host.Address, request.Host.Port, request.Dir)
			file, _ := os.OpenFile(received, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
			file.WriteString(line)
			file.Close()
		})

		if err == nil {
			defer stop()
		}

		time.Sleep(90 * time.Second)
		os.Exit(0)
	}()

	for event := range app.Events {
		if opened, ok := event.(app.URLEvent); ok && opened.URL != nil {
			instance.Deliver([]string{opened.URL.String()})
		}
	}
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "checks":
			os.Exit(checks())
		case "release":
			os.Exit(release())
		}
	}

	serve()
}
