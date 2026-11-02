package placeholder

import (
	"slices"
	"testing"
)

func TestFill(t *testing.T) {
	v := Values{Host: "10.0.0.5", User: "deploy", Port: "2222", Label: "web-01"}
	answers := map[string]string{"Branch": "main"}

	cases := []struct {
		in, want string
	}{
		{"echo {{host}}", "echo 10.0.0.5"},
		{"ssh -p {{ port }} {{USER}}@{{host}}", "ssh -p 2222 deploy@10.0.0.5"},
		{"git checkout {{ask:Branch}} && git pull origin {{ask:Branch}}", "git checkout main && git pull origin main"},
		{"echo {{ask:Missing}}!", "echo !"},
		{"echo {{unknown}} {{label}}", "echo {{unknown}} web-01"},
		{"awk '{{print $1}}' {{host}}", "awk '{{print $1}}' 10.0.0.5"},
		{"echo {{host", "echo {{host"},
		{"{{{host}}}", "{10.0.0.5}"},
	}

	for _, c := range cases {
		if got := Fill(c.in, v, answers); got != c.want {
			t.Errorf("Fill(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestAsks(t *testing.T) {
	got := Asks("deploy {{ask:Branch}} to {{ask: Environment }} ({{ask:Branch}}) on {{host}}")

	if !slices.Equal(got, []string{"Branch", "Environment"}) {
		t.Fatalf("got %q", got)
	}

	if Asks("uptime") != nil {
		t.Fatal("a snippet without questions has no asks")
	}
}

func TestUsesHost(t *testing.T) {
	if !UsesHost("ping {{host}}") || UsesHost("ping {{ask:Target}}") || UsesHost("uptime") {
		t.Fatal("unexpected UsesHost result")
	}
}

func FuzzFill(f *testing.F) {
	f.Add("a {{host}} {{ask:x}} {{ {{ }} }}")

	f.Fuzz(func(t *testing.T, cmd string) {
		Fill(cmd, Values{Host: "h"}, map[string]string{"x": "y"})
		Asks(cmd)
	})
}
