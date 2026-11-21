package mcp

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/northlighthq/boreal/internal/core/access"
	"github.com/northlighthq/boreal/internal/core/config"
	"github.com/northlighthq/boreal/internal/core/sshx"
	"github.com/northlighthq/boreal/internal/core/vault"
)

const (
	dialTimeout  = 20 * time.Second
	maxJumpDepth = 5
)

var (
	ErrNoVault      = errors.New("Boreal isn't set up on this computer yet. Open Boreal once first")
	errUnsharedJump = errors.New("this host connects through a jump host that isn't shared over MCP. Share its folder too, or remove the jump host")
	ErrPassword     = errors.New("Boreal's vault is protected by a master password, which an MCP client can't type. Unlock it with Windows in Boreal › Settings › Security to use MCP")
	errNewHostKey   = errors.New("this server's key isn't trusted yet. Connect to it once from Boreal and accept its key")
	errHostKeyMoved = errors.New("this server's key changed since Boreal last connected, which can mean someone is pretending to be it. Boreal didn't connect. Connect from Boreal, check the new key and accept it only if you expected the change")
)

type keyCheck struct {
	changed bool
}

func (k *keyCheck) reject(i sshx.HostKeyInfo) bool {
	k.changed = k.changed || i.Changed
	return false
}

func (k *keyCheck) err() error {
	if k.changed {
		return errHostKeyMoved
	}

	return errNewHostKey
}

type View interface {
	Policy() vault.MCPPolicy
	Data() vault.Data
	Files(ctx context.Context, h vault.Host) (access.Files, io.Closer, error)
	AgentKeys(h vault.Host) []string
	Run(ctx context.Context, h vault.Host, cmd string, timeout time.Duration) (sshx.RunResult, error)
}

type Backend func() (View, error)

type live struct {
	v        *vault.Vault
	settings config.Settings
}

func Live() (View, error) {
	dir, err := config.Dir()

	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(filepath.Join(dir, vault.FileName)); err != nil {
		return nil, ErrNoVault
	}

	v, err := vault.Open()

	switch {
	case errors.Is(err, vault.ErrNeedUnlock):
		return nil, ErrPassword
	case err != nil:
		return nil, err
	case v.Locked():
		return nil, ErrPassword
	}

	return &live{v: v, settings: config.Load()}, nil
}

func (l *live) Policy() vault.MCPPolicy { return l.v.MCP() }

func (l *live) Data() vault.Data { return l.v.Snapshot() }

func (l *live) AgentKeys(h vault.Host) []string {
	info, err := sshx.InspectAgent(h.Agent)

	if err != nil {
		return nil
	}

	out := make([]string, 0, len(info.Keys))

	for _, k := range info.Keys {
		out = append(out, k.Fingerprint)
	}

	return out
}

func (l *live) target(h vault.Host, depth int) (sshx.Target, error) {
	t := sshx.Target{Host: h}

	if h.Auth == vault.AuthKey && h.KeyID != "" {
		if k, ok := l.v.Key(h.KeyID); ok {
			t.Key = &k
		}
	}

	if h.JumpID != "" && h.JumpID != h.ID && depth < maxJumpDepth {
		if j, ok := l.v.Host(h.JumpID); ok {
			if !l.v.MCP().Allows(j) {
				return t, errUnsharedJump
			}

			jt, err := l.target(j, depth+1)

			if err != nil {
				return t, err
			}

			t.Jump = &jt
		}
	}

	return t, nil
}

type closers []io.Closer

func (c closers) Close() error {
	for i := len(c) - 1; i >= 0; i-- {
		c[i].Close()
	}

	return nil
}

func (l *live) connect(ctx context.Context, h vault.Host) (*sshx.Client, error) {
	ctx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()

	var keys keyCheck
	hooks := sshx.Hooks{ConfirmHostKey: keys.reject, KnownHosts: sshx.SystemHosts{Read: l.settings.SystemKnownHosts}}
	t, err := l.target(h, 0)

	if err != nil {
		return nil, err
	}

	c, err := sshx.Dial(ctx, t, l.v, hooks, 0)

	if errors.Is(err, sshx.ErrHostKeyRejected) {
		return nil, keys.err()
	}

	if err != nil {
		title, hint := sshx.Explain(err, h.Port)
		return nil, errors.New(title + ". " + hint)
	}

	return c, nil
}

func (l *live) Run(ctx context.Context, h vault.Host, cmd string, timeout time.Duration) (sshx.RunResult, error) {
	c, err := l.connect(ctx, h)

	if err != nil {
		return sshx.RunResult{}, err
	}

	defer c.Close()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Run(ctx, cmd, sshx.MaxRunOutput)
}

func (l *live) Files(ctx context.Context, h vault.Host) (access.Files, io.Closer, error) {
	c, err := l.connect(ctx, h)

	if err != nil {
		return nil, nil, err
	}

	s, err := sshx.OpenSFTP(c)

	if err != nil {
		c.Close()
		return nil, nil, errors.New("the server doesn't offer SFTP, which Boreal needs to edit authorized_keys")
	}

	return access.Remote(s.Client), closers{c, s}, nil
}
