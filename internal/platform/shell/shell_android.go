package shell

func Detect() []Profile {
	return []Profile{{ID: "sh", Name: "Shell", Command: []string{"/system/bin/sh"}}}
}
