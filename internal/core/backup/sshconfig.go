package backup

import (
	"strconv"

	"github.com/northlighthq/boreal/internal/core/sshconfig"
	"github.com/northlighthq/boreal/internal/core/vault"
)

func SSHConfig(hosts []vault.Host) []byte {
	aliases := map[string]string{}
	used := map[string]bool{}

	for _, h := range hosts {
		base := sshconfig.Alias(h.Title())
		alias := base

		for n := 2; used[alias]; n++ {
			alias = base + "-" + strconv.Itoa(n)
		}

		used[alias] = true
		aliases[h.ID] = alias
	}

	entries := make([]sshconfig.Entry, 0, len(hosts))

	for _, h := range hosts {
		e := sshconfig.Entry{
			Alias:          aliases[h.ID],
			HostName:       h.Address,
			User:           h.Username,
			Port:           h.Port,
			IdentitiesOnly: h.IdentitiesOnly,
			ForwardAgent:   h.ForwardAgent,
			ProxyCommand:   h.ProxyCommand,
		}

		if j, ok := aliases[h.JumpID]; ok {
			e.Jump = []string{j}
		}

		entries = append(entries, e)
	}

	return []byte(sshconfig.Format(entries))
}
