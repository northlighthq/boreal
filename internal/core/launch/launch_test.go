package launch

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		args             []string
		ok               bool
		kind, user, addr string
		port             int
		dir              string
	}{
		{[]string{"ssh://deploy@example.com:2222"}, true, KindSSH, "deploy", "example.com", 2222, ""},
		{[]string{"ssh://root;fingerprint=SHA256-abc@10.0.0.5"}, true, KindSSH, "root", "10.0.0.5", 22, ""},
		{[]string{"ssh://[2001:db8::1]:22"}, true, KindSSH, "", "2001:db8::1", 22, ""},
		{[]string{"sftp://me@files.example.com/var/log"}, true, KindSFTP, "me", "files.example.com", 22, "/var/log"},
		{[]string{"sftp://me@files.example.com/"}, true, KindSFTP, "me", "files.example.com", 22, ""},
		{[]string{"ssh", "-p", "2200", "admin@db"}, true, KindSSH, "admin", "db", 2200, ""},
		{[]string{"ssh", "web:2201"}, true, KindSSH, "", "web", 2201, ""},
		{[]string{"sftp", "-P", "2022", "u@h:/srv/www"}, true, KindSFTP, "u", "h", 2022, "/srv/www"},
		{[]string{"sftp", "u@[fe80::1]:/tmp"}, true, KindSFTP, "u", "fe80::1", 22, "/tmp"},
		{[]string{"ssh://host:99999"}, false, "", "", "", 0, ""},
		{[]string{"http://example.com"}, false, "", "", "", 0, ""},
		{[]string{"--debug"}, false, "", "", "", 0, ""},
		{nil, false, "", "", "", 0, ""},
	}

	for _, c := range cases {
		r, ok := Parse(c.args)

		if ok != c.ok {
			t.Errorf("Parse(%q) ok = %v", c.args, ok)
			continue
		}

		if !ok {
			continue
		}

		if r.Kind != c.kind || r.Host.Username != c.user || r.Host.Address != c.addr || r.Host.Port != c.port || r.Dir != c.dir {
			t.Errorf("Parse(%q) = %+v %q", c.args, r.Host, r.Dir)
		}
	}
}
