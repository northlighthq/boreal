package settings

import "github.com/northlighthq/boreal/internal/desktop/base"

type async struct {
	done chan error
}

func (x *async) busy() bool {
	return x.done != nil
}

func (x *async) run(a base.Host, f func() error) {
	done := make(chan error, 1)
	x.done = done

	go func() {
		done <- f()
		a.Invalidate()
	}()
}

func (x *async) poll() (error, bool) {
	if x.done == nil {
		return nil, false
	}

	select {
	case err := <-x.done:
		x.done = nil
		return err, true
	default:
		return nil, false
	}
}
