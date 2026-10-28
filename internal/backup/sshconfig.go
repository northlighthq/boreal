package backup

import (
	"strconv"

	"github.com/northlighthq/boreal/internal/sshconfig"
	"github.com/northlighthq/boreal/internal/vault"
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
		entries = append(entries, sshconfig.Entry{
			Alias:    aliases[h.ID],
			HostName: h.Address,
			User:     h.Username,
			Port:     h.Port,
			Jump:     aliases[h.JumpID],
		})
	}

	return []byte(sshconfig.Format(entries))
}
