package instance

import (
	"encoding/json"
	"net"
	"slices"
	"testing"
)

func TestReceive(t *testing.T) {
	for _, c := range []struct {
		token string
		ok    bool
	}{{"secret", true}, {"wrong", false}} {
		client, server := net.Pipe()

		go func() {
			json.NewEncoder(client).Encode(message{Token: c.token, Args: []string{"ssh", "host"}})
			client.Close()
		}()

		args, err := receive(server, "secret")
		server.Close()

		if (err == nil) != c.ok {
			t.Fatalf("token %q: err = %v", c.token, err)
		}

		if c.ok && !slices.Equal(args, []string{"ssh", "host"}) {
			t.Fatalf("args = %q", args)
		}
	}
}
