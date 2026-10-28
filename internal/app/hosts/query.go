package hosts

import "github.com/northlighthq/boreal/internal/vault"

func (p *Panel) Collapsed(folder string) bool {
	return p.collapsed[folder]
}

func (p *Panel) EditingFolder() bool {
	return p.edit.active
}

func (p *Panel) Hosts() []vault.Host {
	return p.hosts
}

func (p *Panel) ItemCount() int {
	return len(p.items())
}

func (p *Panel) MoreMenuOpen() bool {
	return p.moreMenu.IsOpen()
}
