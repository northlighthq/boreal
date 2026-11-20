package mcp

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	Command   = "mcp"
	logPrefix = "boreal mcp: "
)

func Main() int {
	view, err := Live()

	if err != nil {
		fmt.Fprintln(os.Stderr, logPrefix+err.Error())
		return 1
	}

	if !view.Policy().Enabled {
		fmt.Fprintln(os.Stderr, logPrefix+errOff.Error())
		return 1
	}

	log, err := NewFileLog()

	if err != nil {
		fmt.Fprintln(os.Stderr, logPrefix+err.Error())
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := New(Live, log).MCP().Run(ctx, &mcp.StdioTransport{}); err != nil && ctx.Err() == nil {
		fmt.Fprintln(os.Stderr, logPrefix+err.Error())
		return 1
	}

	return 0
}
