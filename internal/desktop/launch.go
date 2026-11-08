package desktop

import (
	"gioui.org/io/system"

	"github.com/northlighthq/boreal/internal/core/launch"
	"github.com/northlighthq/boreal/internal/core/vault"
	"github.com/northlighthq/boreal/internal/platform/instance"
)

func (a *App) startLaunches(args []string) {
	if r, ok := launch.Parse(args); ok {
		a.launches <- r
	}

	stop, err := instance.Listen(func(args []string) {
		r, ok := launch.Parse(args)

		if !ok {
			return
		}

		select {
		case a.launches <- r:
		default:
		}

		a.win.Perform(system.ActionRaise)
		a.Invalidate()
	})

	if err == nil {
		a.stopListen = stop
	}
}

func (a *App) handleLaunches() {
	for {
		select {
		case r := <-a.launches:
			a.openLaunch(r)
		default:
			return
		}
	}
}

func (a *App) openLaunch(r launch.Request) {
	h := a.savedHost(r.Host)
	p := a.newSSHPane(h)
	a.ws.addTab(p)

	if r.Kind != launch.KindSFTP {
		return
	}

	if r.Dir != "" {
		a.tools.sftp.Want(p.sess.ID, r.Dir)
	}

	a.tools.show(a, a.tools.sftp)
}

func (a *App) savedHost(h vault.Host) vault.Host {
	for _, saved := range a.vault.Snapshot().Hosts {
		if saved.Address == h.Address && (h.Username == "" || saved.Username == h.Username) && (h.Port == 22 || saved.Port == h.Port) {
			return saved
		}
	}

	h.Label = h.Address
	return h
}
