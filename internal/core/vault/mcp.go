package vault

import (
	"slices"
	"strings"
)

const RootFolder = "/"

type MCPPolicy struct {
	Enabled    bool     `json:"enabled,omitempty"`
	Read       bool     `json:"read,omitempty"`
	Access     bool     `json:"access,omitempty"`
	Execute    bool     `json:"execute,omitempty"`
	Files      bool     `json:"files,omitempty"`
	WriteFiles bool     `json:"writeFiles,omitempty"`
	AllHosts   bool     `json:"allHosts,omitempty"`
	Folders    []string `json:"folders,omitempty"`
}

func (p MCPPolicy) Allows(h Host) bool {
	if !p.Enabled {
		return false
	}

	if p.AllHosts {
		return true
	}

	for _, f := range p.Folders {
		switch {
		case f == RootFolder && h.Group == "":
			return true
		case f != RootFolder && f != "" && (h.Group == f || strings.HasPrefix(h.Group, f+FolderSep)):
			return true
		}
	}

	return false
}

func (p MCPPolicy) Scoped() bool {
	return p.AllHosts || len(p.Folders) > 0
}

func (v *Vault) MCP() MCPPolicy {
	v.mu.RLock()
	defer v.mu.RUnlock()

	p := v.data.MCP
	p.Folders = slices.Clone(p.Folders)
	return p
}

func (v *Vault) SetMCP(p MCPPolicy) error {
	p.Folders = slices.Compact(slices.Sorted(slices.Values(p.Folders)))

	return v.update(func(d *Data) {
		d.MCP = p
	})
}
