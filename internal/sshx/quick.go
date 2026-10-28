package sshx

import (
	"strconv"
	"strings"

	"github.com/northlighthq/boreal/internal/vault"
)

func ParseQuickConnect(s string) (vault.Host, bool) {
	fields := strings.Fields(strings.TrimSpace(s))

	if len(fields) > 0 && fields[0] == "ssh" {
		fields = fields[1:]
	}

	h := vault.Host{Port: 22, Auth: vault.AuthAgent}

	for i := 0; i < len(fields); i++ {
		f := fields[i]

		switch {
		case f == "-p" && i+1 < len(fields):
			h.Port, _ = strconv.Atoi(fields[i+1])
			i++
		case f == "-l" && i+1 < len(fields):
			h.Username = fields[i+1]
			i++
		case strings.HasPrefix(f, "-"):
		default:
			target := f

			if u, rest, ok := strings.Cut(target, "@"); ok {
				h.Username, target = u, rest
			}

			if host, port, ok := strings.Cut(target, ":"); ok && !strings.Contains(port, ":") {
				target = host
				h.Port, _ = strconv.Atoi(port)
			}

			h.Address = target
		}
	}

	if h.Port == 0 {
		h.Port = 22
	}

	return h, h.Address != ""
}
