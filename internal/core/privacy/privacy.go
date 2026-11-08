package privacy

import "regexp"

var (
	ipv4 = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	ipv6 = regexp.MustCompile(`\b(?:[0-9A-Fa-f]{1,4}:){2,7}[0-9A-Fa-f]{1,4}\b`)
)

var keep = map[string]struct{}{"127.0.0.1": {}, "0.0.0.0": {}}

func RedactIPs(s string) string {
	s = ipv4.ReplaceAllStringFunc(s, func(ip string) string {
		if _, ok := keep[ip]; ok {
			return ip
		}

		return "•••.•••.•••.•••"
	})

	return ipv6.ReplaceAllString(s, "••••:••••::••••")
}
