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
)

type View interface {
	Policy() vault.MCPPolicy
	Data() vault.Data
	Files(ctx context.Context, h vault.Host) (access.Files, io.Closer, error)
	AgentKeys(h vault.Host) []string
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

func (l *live) Files(ctx context.Context, h vault.Host) (access.Files, io.Closer, error) {
	ctx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()

	hooks := sshx.Hooks{KnownHosts: sshx.SystemHosts{Read: l.settings.SystemKnownHosts}}
	t, err := l.target(h, 0)

	if err != nil {
		return nil, nil, err
	}

	c, err := sshx.Dial(ctx, t, l.v, hooks, 0)

	if errors.Is(err, sshx.ErrHostKeyRejected) {
		return nil, nil, errors.New("this server's key isn't trusted yet. Connect to it once from Boreal and accept its key")
	}

	if err != nil {
		title, hint := sshx.Explain(err, h.Port)
		return nil, nil, errors.New(title + ". " + hint)
	}

	s, err := sshx.OpenSFTP(c)

	if err != nil {
		c.Close()
		return nil, nil, errors.New("the server doesn't offer SFTP, which Boreal needs to edit authorized_keys")
	}

	return access.Remote(s.Client), closers{c, s}, nil
}
