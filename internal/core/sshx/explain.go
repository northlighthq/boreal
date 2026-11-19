package sshx

import (
	"errors"
	"strconv"
	"strings"
)

func Explain(err error, port int) (title, hint string) {
	if err == nil {
		return "Couldn't connect", "Something went wrong while opening the session."
	}

	msg := strings.ToLower(err.Error())
	has := func(parts ...string) bool {
		for _, p := range parts {
			if strings.Contains(msg, p) {
				return true
			}
		}

		return false
	}

	switch {
	case errors.Is(err, ErrHostKeyRejected):
		return "Server key rejected", "The server's identity wasn't trusted, so Boreal didn't connect."
	case has("timeout", "timed out", "deadline exceeded"):
		return "The server didn't answer", "Check the address and that the server is online and reachable from this network."
	case has("connection refused"):
		return "Connection refused", "Nothing is accepting connections on port " + strconv.Itoa(port) + ". Is SSH running there?"
	case has("no such host", "server misbehaving", "name resolution"):
		return "Can't find this server", "The address didn't resolve. Check it for typos, or that you're on the right network or VPN."
	case has("network is unreachable", "no route to host", "host is unreachable"):
		return "Network unreachable", "This device can't reach the server. Check your connection or VPN."
	case has("unable to authenticate", "permission denied", "auth"):
		return "Sign-in failed", "The server refused the credentials. Check the username, password or key."
	case has("reset by peer", "eof", "broken pipe"):
		return "The server closed the connection", "It hung up during the handshake. Try again in a moment."
	}

	return "Couldn't connect", "The connection failed before the session started."
}
