package backup

import (
	"github.com/northlighthq/boreal/internal/core/sshx"
	"github.com/northlighthq/boreal/internal/core/vault"
)

func Defang(d *vault.Data) []string {
	var touched []string

	for i := range d.Hosts {
		h := &d.Hosts[i]

		if h.ProxyCommand == "" && h.StartupCommand == "" {
			continue
		}

		h.ProxyCommand, h.StartupCommand = "", ""
		touched = append(touched, h.Title())
	}

	for i := range d.Forwards {
		f := &d.Forwards[i]

		if !f.OnStart && sshx.Loopback(f.BindAddr) {
			continue
		}

		f.OnStart = false

		if !sshx.Loopback(f.BindAddr) {
			f.BindAddr = ""
		}

		touched = append(touched, f.Label)
	}

	return touched
}
