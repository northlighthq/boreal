package sshx

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const (
	AgentNone        = "none"
	agentEnv         = "SSH_AUTH_SOCK"
	windowsAgentPipe = `\\.\pipe\openssh-ssh-agent`
	localPipePrefix  = `\\.\pipe\`
	pageantName      = "Pageant"
	maxAgentTries    = 5
)

var knownAgents = map[string][]string{
	"darwin": {
		"Library/Group Containers/2BUA8C4S2C.com.1password/t/agent.sock",
		"Library/Containers/com.bitwarden.desktop/Data/.bitwarden-ssh-agent.sock",
	},
	"linux": {
		".1password/agent.sock",
		".bitwarden-ssh-agent.sock",
	},
}

var (
	ErrNoAgent     = errors.New("no SSH agent found")
	ErrAgentOff    = errors.New("the SSH agent is turned off for this host")
	ErrRemoteAgent = errors.New("the SSH agent must be on this computer")
)

type AgentKey struct {
	Type        string
	Comment     string
	Fingerprint string
}

type AgentInfo struct {
	Path string
	Keys []AgentKey
}

func LocalAgentPath(p string) bool {
	p = strings.TrimSpace(p)

	if strings.HasPrefix(p, "//") {
		return false
	}

	if !strings.HasPrefix(p, `\`) {
		return true
	}

	return localPipe(p)
}

func localPipe(p string) bool {
	return len(p) > len(localPipePrefix) && strings.EqualFold(p[:len(localPipePrefix)], localPipePrefix)
}

func expandAgentPath(p string) string {
	p = strings.TrimSpace(p)

	if p == agentEnv || p == "$"+agentEnv {
		return os.Getenv(agentEnv)
	}

	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, rest)
		}
	}

	if localPipe(p) {
		return p
	}

	return os.ExpandEnv(p)
}

func agentCandidates(want string) ([]string, error) {
	want = strings.TrimSpace(want)

	if strings.EqualFold(want, AgentNone) {
		return nil, ErrAgentOff
	}

	if want != "" {
		p := expandAgentPath(want)

		if p == "" {
			return nil, ErrNoAgent
		}

		if !LocalAgentPath(p) {
			return nil, ErrRemoteAgent
		}

		return []string{p}, nil
	}

	if runtime.GOOS == "windows" {
		return []string{windowsAgentPipe, pageantName}, nil
	}

	var out []string

	if sock := os.Getenv(agentEnv); sock != "" {
		out = append(out, sock)
	}

	home, err := os.UserHomeDir()

	if err != nil {
		return out, nil
	}

	for _, rel := range knownAgents[runtime.GOOS] {
		p := filepath.Join(home, rel)

		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}

	return out, nil
}

func openAgentPath(p string) (io.ReadWriteCloser, error) {
	if p == pageantName {
		return dialPageant()
	}

	if runtime.GOOS == "windows" && localPipe(p) {
		return os.OpenFile(p, os.O_RDWR, 0)
	}

	return net.Dial("unix", p)
}

func openAgent(want string) (io.ReadWriteCloser, string, error) {
	paths, err := agentCandidates(want)

	if err != nil {
		return nil, "", err
	}

	err = ErrNoAgent

	for _, p := range paths {
		rw, dialErr := openAgentPath(p)

		if dialErr == nil {
			return rw, p, nil
		}

		err = fmt.Errorf("%s: %w", p, dialErr)
	}

	return nil, "", err
}

func dialAgent(want string) (agent.ExtendedAgent, func(), error) {
	rw, _, err := openAgent(want)

	if err != nil {
		return nil, nil, err
	}

	return agent.NewClient(rw), func() { rw.Close() }, nil
}

func AgentName(p string) string {
	lower := strings.ToLower(p)

	switch {
	case p == pageantName:
		return pageantName
	case strings.Contains(lower, "1password"):
		return "1Password"
	case strings.Contains(lower, "bitwarden"):
		return "Bitwarden"
	case strings.EqualFold(p, windowsAgentPipe):
		return "the OpenSSH agent pipe"
	}

	return p
}

func AgentPath(want string) (string, bool) {
	rw, p, err := openAgent(want)

	if err != nil {
		return "", false
	}

	rw.Close()
	return p, true
}

func InspectAgent(want string) (AgentInfo, error) {
	rw, p, err := openAgent(want)

	if err != nil {
		return AgentInfo{}, err
	}

	defer rw.Close()
	keys, err := agent.NewClient(rw).List()

	if err != nil {
		return AgentInfo{Path: p}, err
	}

	info := AgentInfo{Path: p, Keys: make([]AgentKey, 0, len(keys))}

	for _, k := range keys {
		info.Keys = append(info.Keys, AgentKey{Type: k.Type(), Comment: k.Comment, Fingerprint: ssh.FingerprintSHA256(k)})
	}

	return info, nil
}

func agentSigners(ag agent.Agent, want string, progress func(string)) func() ([]ssh.Signer, error) {
	return func() ([]ssh.Signer, error) {
		signers, err := ag.Signers()

		if err != nil {
			return nil, err
		}

		if want == "" {
			if len(signers) > maxAgentTries {
				progress(fmt.Sprintf("The agent has %d keys and servers usually stop after 6 tries. If this fails, pick one in the host editor.", len(signers)))
			}

			return signers, nil
		}

		for _, s := range signers {
			if ssh.FingerprintSHA256(s.PublicKey()) == want {
				return []ssh.Signer{s}, nil
			}
		}

		progress("The key chosen for this host isn't in the agent. Unlock the agent or pick another key in the host editor.")
		return nil, nil
	}
}
