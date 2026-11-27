//go:build !android

package config

func privateDir() (string, bool) {
	return "", false
}
