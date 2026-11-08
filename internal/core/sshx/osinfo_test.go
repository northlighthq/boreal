package sshx

import "testing"

func TestParseOS(t *testing.T) {
	cases := []struct {
		name, out, want string
	}{
		{"ubuntu", "Linux\nNAME=\"Ubuntu\"\nID=ubuntu\nID_LIKE=debian\n", "ubuntu"},
		{"quoted id", "Linux\nID=\"rhel\"\nID_LIKE=\"fedora\"\n", "rhel"},
		{"derivative falls back to its family", "Linux\nID=zorin\nID_LIKE=\"ubuntu debian\"\n", "ubuntu"},
		{"amazon linux keeps the generic logo", "Linux\nID=\"amzn\"\nID_LIKE=\"centos rhel fedora\"\n", "linux"},
		{"unknown distro", "Linux\nID=somethingelse\n", "linux"},
		{"no os-release", "Linux\n", "linux"},
		{"freebsd", "FreeBSD\n", "freebsd"},
		{"macos", "Darwin\n", "macos"},
		{"unknown system", "Welcome to the switch\n", ""},
		{"empty", "", ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseOS(c.out); got != c.want {
				t.Fatalf("parseOS = %q, want %q", got, c.want)
			}
		})
	}
}
