package sshx

import (
	"bufio"
	"io"
	"strings"
	"time"
)

const (
	maxOSOutput = 16 * 1024
	osTimeout   = 10 * time.Second
	osCommand   = `uname -s 2>/dev/null; cat /etc/os-release 2>/dev/null`

	osGeneric = "linux"
)

var osNames = map[string]string{
	"almalinux":           "almalinux",
	"alpine":              "alpine",
	"amzn":                osGeneric,
	"arch":                "arch",
	"archarm":             "arch",
	"centos":              "centos",
	"debian":              "debian",
	"fedora":              "fedora",
	"gentoo":              "gentoo",
	"kali":                "kali",
	"linuxmint":           "linuxmint",
	"manjaro":             "manjaro",
	"nixos":               "nixos",
	"ol":                  osGeneric,
	"openwrt":             "openwrt",
	"opensuse":            "opensuse",
	"opensuse-leap":       "opensuse",
	"opensuse-tumbleweed": "opensuse",
	"pop":                 "pop",
	"raspbian":            "raspbian",
	"rhel":                "rhel",
	"rocky":               "rocky",
	"sles":                "opensuse",
	"suse":                "opensuse",
	"ubuntu":              "ubuntu",
}

var kernelNames = map[string]string{
	"Darwin":  "macos",
	"FreeBSD": "freebsd",
	"Linux":   osGeneric,
}

// OSNames lists every value DetectOS can return.
func OSNames() []string {
	var out []string

	for _, m := range []map[string]string{osNames, kernelNames} {
		for _, name := range m {
			out = append(out, name)
		}
	}

	return out
}

// DetectOS asks the server what it runs. It returns "" for a system it doesn't know.
func DetectOS(c *Client) (string, error) {
	sess, err := c.NewSession()

	if err != nil {
		return "", err
	}

	defer sess.Close()
	stop := time.AfterFunc(osTimeout, func() { sess.Close() })
	defer stop.Stop()

	stdout, err := sess.StdoutPipe()

	if err != nil {
		return "", err
	}

	if err := sess.Start(osCommand); err != nil {
		return "", err
	}

	out, err := io.ReadAll(io.LimitReader(stdout, maxOSOutput))

	if err != nil && len(out) == 0 {
		return "", err
	}

	return parseOS(string(out)), nil
}

func parseOS(out string) string {
	var id, like, kernel string
	sc := bufio.NewScanner(strings.NewReader(out))

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())

		switch {
		case strings.HasPrefix(line, "ID="):
			id = osValue(line, "ID=")
		case strings.HasPrefix(line, "ID_LIKE="):
			like = osValue(line, "ID_LIKE=")
		case kernel == "" && kernelNames[line] != "":
			kernel = kernelNames[line]
		}
	}

	if name, ok := osNames[id]; ok {
		return name
	}

	for _, l := range strings.Fields(like) {
		if name, ok := osNames[l]; ok {
			return name
		}
	}

	return kernel
}

func osValue(line, prefix string) string {
	return strings.ToLower(strings.Trim(strings.TrimPrefix(line, prefix), `"'`))
}
