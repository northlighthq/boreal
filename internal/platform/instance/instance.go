package instance

import (
	"bufio"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/northlighthq/boreal/internal/core/config"
)

const (
	stateFile   = "instance.json"
	loopback    = "127.0.0.1"
	dialTimeout = 2 * time.Second
	readTimeout = 5 * time.Second
	maxMessage  = 64 << 10
	reply       = "ok\n"
)

var errBadToken = errors.New("instance: bad token")

type state struct {
	Port  int    `json:"port"`
	Token string `json:"token"`
}

type message struct {
	Token string   `json:"token"`
	Args  []string `json:"args"`
}

func path() (string, error) {
	dir, err := config.Dir()

	if err != nil {
		return "", err
	}

	return filepath.Join(dir, stateFile), nil
}

func Send(args []string) bool {
	p, err := path()

	if err != nil {
		return false
	}

	raw, err := os.ReadFile(p)

	if err != nil {
		return false
	}

	var st state

	if json.Unmarshal(raw, &st) != nil || st.Port == 0 {
		return false
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort(loopback, strconv.Itoa(st.Port)), dialTimeout)

	if err != nil {
		return false
	}

	defer conn.Close()
	conn.SetDeadline(time.Now().Add(readTimeout))
	allowForeground()

	if json.NewEncoder(conn).Encode(message{Token: st.Token, Args: args}) != nil {
		return false
	}

	line, err := bufio.NewReader(conn).ReadString('\n')
	return err == nil && line == reply
}

func Listen(handle func(args []string)) (func(), error) {
	p, err := path()

	if err != nil {
		return nil, err
	}

	l, err := net.Listen("tcp", net.JoinHostPort(loopback, "0"))

	if err != nil {
		return nil, err
	}

	var b [16]byte
	rand.Read(b[:])
	st := state{Port: l.Addr().(*net.TCPAddr).Port, Token: hex.EncodeToString(b[:])}
	raw, _ := json.Marshal(st)

	if err := config.WriteAtomic(p, raw); err != nil {
		l.Close()
		return nil, err
	}

	go serve(l, st.Token, handle)

	return func() {
		l.Close()

		if cur, err := os.ReadFile(p); err == nil && string(cur) == string(raw) {
			os.Remove(p)
		}
	}, nil
}

func serve(l net.Listener, token string, handle func([]string)) {
	for {
		conn, err := l.Accept()

		if err != nil {
			return
		}

		go func() {
			defer conn.Close()

			args, err := receive(conn, token)

			if err != nil {
				return
			}

			io.WriteString(conn, reply)
			handle(args)
		}()
	}
}

func receive(conn net.Conn, token string) ([]string, error) {
	conn.SetDeadline(time.Now().Add(readTimeout))

	var m message

	if err := json.NewDecoder(io.LimitReader(conn, maxMessage)).Decode(&m); err != nil {
		return nil, err
	}

	if subtle.ConstantTimeCompare([]byte(m.Token), []byte(token)) != 1 {
		return nil, errBadToken
	}

	return m.Args, nil
}
