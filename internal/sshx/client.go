package sshx

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/northlighthq/boreal/internal/vault"
)

const (
	dialTimeout      = 15 * time.Second
	windowsAgentPipe = `\\.\pipe\openssh-ssh-agent`
	termType         = "xterm-256color"
)

var ErrHostKeyRejected = errors.New("server key rejected")

type HostKeyInfo struct {
	Host           string
	KeyType        string
	Fingerprint    string
	Changed        bool
	OldFingerprint string
}

type Hooks struct {
	ConfirmHostKey func(HostKeyInfo) bool
	AskPassword    func(prompt string) (string, bool)
	KeyPassphrase  func(k vault.Key) (string, bool)
	Progress       func(msg string)
}

type Target struct {
	Host vault.Host
	Key  *vault.Key
	Jump *Target
}

type Client struct {
	*ssh.Client
	jump    *Client
	stop    chan struct{}
	closeMu sync.Once

	agentOnce  sync.Once
	agentErr   error
	agentClose func()
}

func (c *Client) Close() error {
	c.closeMu.Do(func() {
		close(c.stop)
		c.Client.Close()

		if c.jump != nil {
			c.jump.Close()
		}

		if c.agentClose != nil {
			c.agentClose()
		}
	})
	return nil
}

func Dial(ctx context.Context, t Target, v *vault.Vault, hooks Hooks, keepAlive time.Duration) (*Client, error) {
	progress := hooks.Progress

	if progress == nil {
		progress = func(string) {}
	}

	addr := net.JoinHostPort(t.Host.Address, strconv.Itoa(port(t.Host)))

	var jump *Client
	var conn net.Conn
	var err error

	if t.Jump != nil {
		progress(fmt.Sprintf("Connecting to jump host %s…", t.Jump.Host.Title()))
		jump, err = Dial(ctx, *t.Jump, v, hooks, keepAlive)

		if err != nil {
			return nil, fmt.Errorf("jump host %s: %w", t.Jump.Host.Title(), err)
		}

		progress(fmt.Sprintf("Opening tunnel to %s…", addr))
		conn, err = jump.Dial("tcp", addr)
	} else {
		progress(fmt.Sprintf("Connecting to %s…", addr))
		d := net.Dialer{Timeout: dialTimeout}
		conn, err = d.DialContext(ctx, "tcp", addr)
	}

	if err != nil {
		if jump != nil {
			jump.Close()
		}

		return nil, err
	}

	auths, cleanup := authMethods(t, hooks)
	defer cleanup()

	cfg := &ssh.ClientConfig{
		User:            username(t.Host),
		Auth:            auths,
		HostKeyCallback: hostKeyCallback(v, hooks),
		Timeout:         dialTimeout,
	}

	progress("Authenticating…")
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)

	if err != nil {
		conn.Close()

		if jump != nil {
			jump.Close()
		}

		return nil, err
	}

	client := &Client{Client: ssh.NewClient(c, chans, reqs), jump: jump, stop: make(chan struct{})}

	if keepAlive > 0 {
		go client.keepAlive(keepAlive)
	}

	return client, nil
}

func (c *Client) keepAlive(every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			if _, _, err := c.SendRequest("keepalive@openssh.com", true, nil); err != nil {
				c.Close()
				return
			}
		case <-c.stop:
			return
		}
	}
}

func port(h vault.Host) int {
	if h.Port == 0 {
		return 22
	}

	return h.Port
}

func username(h vault.Host) string {
	if h.Username != "" {
		return h.Username
	}

	if u := os.Getenv("USERNAME"); u != "" {
		return u
	}

	return os.Getenv("USER")
}

func authMethods(t Target, hooks Hooks) ([]ssh.AuthMethod, func()) {
	var methods []ssh.AuthMethod
	cleanup := func() {}

	progress := func(msg string) {
		if hooks.Progress != nil {
			hooks.Progress(msg)
		}
	}

	keyTried, warned := false, false

	if t.Host.Username == "" {
		progress(fmt.Sprintf("No username set for this host: logging in as “%s”. Edit the host to set the right user.", username(t.Host)))
	}

	switch t.Host.Auth {
	case vault.AuthKey:
		switch {
		case t.Key == nil:
			progress("This host has no key selected: edit it and choose one in Keychain.")
		default:
			signers, err := Signers(*t.Key, hooks)

			if err != nil {
				progress(fmt.Sprintf("Couldn't load the key “%s”: %v", t.Key.Label, err))
				break
			}

			progress(fmt.Sprintf("Trying key “%s”…", t.Key.Label))
			methods = append(methods, ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
				keyTried = true
				return signers, nil
			}))
		}
	case vault.AuthPassword:
		if t.Host.Password != "" {
			methods = append(methods, ssh.Password(t.Host.Password))
		}
	}

	if ag, closeFn, err := dialAgent(); err == nil {
		methods = append(methods, ssh.PublicKeysCallback(ag.Signers))
		cleanup = closeFn
	}

	if hooks.AskPassword != nil {
		ask := func() (string, error) {
			if keyTried && !warned {
				warned = true
				progress(fmt.Sprintf("The server didn't accept the key “%s”. Add its public key to ~/.ssh/authorized_keys on the server.", t.Key.Label))
			}

			pw, ok := hooks.AskPassword(fmt.Sprintf("Password for %s@%s", username(t.Host), t.Host.Address))

			if !ok {
				return "", errors.New("cancelled")
			}

			return pw, nil
		}
		methods = append(methods,
			ssh.RetryableAuthMethod(ssh.PasswordCallback(ask), 3),
			ssh.RetryableAuthMethod(ssh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))

				for i, q := range questions {
					a, ok := hooks.AskPassword(q)

					if !ok {
						return nil, errors.New("cancelled")
					}

					answers[i] = a
				}

				return answers, nil
			}), 3),
		)
	}

	return methods, cleanup
}

func dialAgent() (agent.ExtendedAgent, func(), error) {
	var rw io.ReadWriteCloser
	var err error

	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" && runtime.GOOS != "windows" {
		rw, err = net.Dial("unix", sock)
	} else if runtime.GOOS == "windows" {
		rw, err = os.OpenFile(windowsAgentPipe, os.O_RDWR, 0)
	} else {
		err = errors.New("no agent")
	}

	if err != nil {
		return nil, nil, err
	}

	return agent.NewClient(rw), func() { rw.Close() }, nil
}

func AgentAvailable() bool {
	_, closeFn, err := dialAgent()

	if err != nil {
		return false
	}

	closeFn()
	return true
}

func hostKeyCallback(v *vault.Vault, hooks Hooks) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		fp := ssh.FingerprintSHA256(key)
		encoded := base64.StdEncoding.EncodeToString(key.Marshal())
		known, ok := v.KnownHost(hostname)

		if ok && known.Key == encoded {
			return nil
		}

		info := HostKeyInfo{Host: hostname, KeyType: key.Type(), Fingerprint: fp}

		if ok {
			info.Changed = true
			info.OldFingerprint = known.Fingerprint
		}

		if hooks.ConfirmHostKey == nil || !hooks.ConfirmHostKey(info) {
			return ErrHostKeyRejected
		}

		return v.TrustHost(vault.KnownHost{Host: hostname, KeyType: key.Type(), Key: encoded, Fingerprint: fp})
	}
}

func User(h vault.Host) string { return username(h) }
