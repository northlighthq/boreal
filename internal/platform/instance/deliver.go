package instance

import "sync"

var local struct {
	mu      sync.Mutex
	handle  func([]string)
	pending [][]string
}

// Deliver hands args to the running instance's handler, queueing them until Listen is called.
func Deliver(args []string) {
	local.mu.Lock()
	handle := local.handle

	if handle == nil {
		local.pending = append(local.pending, args)
	}

	local.mu.Unlock()

	if handle != nil {
		handle(args)
	}
}

func setLocal(handle func([]string)) {
	local.mu.Lock()
	local.handle = handle
	pending := local.pending
	local.pending = nil
	local.mu.Unlock()

	for _, args := range pending {
		handle(args)
	}
}
