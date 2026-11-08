package sshx

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

var ErrHostKeyRevoked = errors.New("this server key is marked as revoked in known_hosts")

type SystemHosts struct {
	Read  bool
	Write bool
}

type systemVerdict int

const (
	systemUnknown systemVerdict = iota
	systemTrusted
	systemChanged
	systemRevoked
)

func UserKnownHostsFile() string {
	home, err := os.UserHomeDir()

	if err != nil {
		return ""
	}

	return filepath.Join(home, ".ssh", "known_hosts")
}

func globalKnownHostsFile() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("ProgramData"), "ssh", "ssh_known_hosts")
	}

	return "/etc/ssh/ssh_known_hosts"
}

func knownHostsFiles() []string {
	var files []string

	for _, f := range []string{UserKnownHostsFile(), globalKnownHostsFile()} {
		if f == "" {
			continue
		}

		if _, err := os.Stat(f); err == nil {
			files = append(files, f)
		}
	}

	return files
}

func checkSystem(hostname string, remote net.Addr, key ssh.PublicKey) (systemVerdict, ssh.PublicKey) {
	files := knownHostsFiles()

	if len(files) == 0 {
		return systemUnknown, nil
	}

	cb, err := knownhosts.New(files...)

	if err != nil {
		return systemUnknown, nil
	}

	if _, ok := remote.(*net.TCPAddr); !ok {
		remote = &net.TCPAddr{IP: net.IPv4zero}
	}

	err = cb(hostname, remote, key)

	if err == nil {
		return systemTrusted, nil
	}

	var revoked *knownhosts.RevokedError

	if errors.As(err, &revoked) {
		return systemRevoked, nil
	}

	var keyErr *knownhosts.KeyError

	if errors.As(err, &keyErr) && len(keyErr.Want) > 0 {
		return systemChanged, keyErr.Want[0].Key
	}

	return systemUnknown, nil
}

func addToKnownHosts(hostname string, key ssh.PublicKey) error {
	path := UserKnownHostsFile()

	if path == "" {
		return errors.New("no home folder")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)

	if err != nil {
		return err
	}

	defer f.Close()

	line := knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key)

	if needsNewline(path) {
		line = "\n" + line
	}

	_, err = f.WriteString(line + "\n")
	return err
}

func needsNewline(path string) bool {
	raw, err := os.ReadFile(path)
	return err == nil && len(raw) > 0 && raw[len(raw)-1] != '\n'
}
