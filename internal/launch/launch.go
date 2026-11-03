package launch

import (
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/northlighthq/boreal/internal/sshx"
	"github.com/northlighthq/boreal/internal/vault"
)

const (
	KindSSH  = "ssh"
	KindSFTP = "sftp"
)

type Request struct {
	Kind string     `json:"kind"`
	Host vault.Host `json:"host"`
	Dir  string     `json:"dir,omitempty"`
}

func Parse(args []string) (Request, bool) {
	if len(args) == 0 {
		return Request{}, false
	}

	first := strings.TrimSpace(args[0])

	if u, err := url.Parse(first); err == nil && (u.Scheme == KindSSH || u.Scheme == KindSFTP) && u.Host != "" {
		return fromURL(u)
	}

	switch strings.ToLower(first) {
	case KindSSH:
		h, ok := sshx.ParseQuickConnect(strings.Join(args[1:], " "))
		return Request{Kind: KindSSH, Host: h}, ok
	case KindSFTP:
		return sftpArgs(args[1:])
	}

	return Request{}, false
}

func fromURL(u *url.URL) (Request, bool) {
	r := Request{Kind: u.Scheme, Host: vault.Host{Address: u.Hostname(), Port: 22, Auth: vault.AuthAgent}}

	if u.User != nil {
		r.Host.Username, _, _ = strings.Cut(u.User.Username(), ";")
	}

	if p := u.Port(); p != "" {
		n, err := strconv.Atoi(p)

		if err != nil || n <= 0 || n > 65535 {
			return Request{}, false
		}

		r.Host.Port = n
	}

	if r.Kind == KindSFTP {
		r.Dir = cleanDir(u.Path)
	}

	return r, r.Host.Address != ""
}

func sftpArgs(args []string) (Request, bool) {
	r := Request{Kind: KindSFTP, Host: vault.Host{Port: 22, Auth: vault.AuthAgent}}

	for i := 0; i < len(args); i++ {
		a := args[i]

		switch {
		case (a == "-P" || a == "-p") && i+1 < len(args):
			r.Host.Port, _ = strconv.Atoi(args[i+1])
			i++
		case strings.HasPrefix(a, "-"):
		default:
			target := a

			if user, rest, ok := strings.Cut(target, "@"); ok {
				r.Host.Username, target = user, rest
			}

			host, dir := splitRemote(target)
			r.Host.Address, r.Dir = host, dir
		}
	}

	if r.Host.Port <= 0 || r.Host.Port > 65535 {
		r.Host.Port = 22
	}

	return r, r.Host.Address != ""
}

func splitRemote(s string) (host, dir string) {
	if strings.HasPrefix(s, "[") {
		end := strings.Index(s, "]")

		if end < 0 {
			return s, ""
		}

		host, rest := s[1:end], s[end+1:]
		_, dir, _ = strings.Cut(rest, ":")
		return host, cleanDir(dir)
	}

	if net.ParseIP(s) != nil {
		return s, ""
	}

	host, dir, _ = strings.Cut(s, ":")
	return host, cleanDir(dir)
}

func cleanDir(p string) string {
	if p == "" || p == "/" {
		return ""
	}

	return p
}
