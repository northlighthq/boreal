package core

import (
	"strconv"

	"gioui.org/layout"
	"gioui.org/x/explorer"

	"github.com/northlighthq/boreal/internal/config"
	"github.com/northlighthq/boreal/internal/platform/shell"
	"github.com/northlighthq/boreal/internal/session"
	"github.com/northlighthq/boreal/internal/transfer"
	"github.com/northlighthq/boreal/internal/tunnel"
	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/ui/toast"
	"github.com/northlighthq/boreal/internal/vault"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

type Side int

const (
	Left Side = iota
	Right
)

type Modal interface {
	Layout(gtx C) bool
}

type Docker interface {
	Dialog() *kit.Dialog
}

type Tool interface {
	Title() string
	Icon() *icons.Icon
	Side() Side
	Shortcut() string
	Actions(gtx C) D
	Body(gtx C) D
	Refresh()
}

type Host interface {
	Kit() *kit.Kit
	Vault() *vault.Vault
	Toasts() *toast.Stack
	Explorer() *explorer.Explorer
	Tunnels() *tunnel.Manager
	Transfers() *transfer.Queue
	Invalidate()
	TitleBar(gtx C) D

	Settings() config.Settings
	UpdateSettings(s config.Settings)
	SetTheme(name string)
	Profiles() []shell.Profile
	DefaultProfile() shell.Profile

	Open(m Modal)
	OpenDocked(s Side, m Modal)
	OpenIn(t Tool, m Modal)
	CloseDocked(s Side)
	ShowTool(t Tool)
	ShowPalette()
	AskPassword(title, message string) (string, bool)
	AskKeyPassphrase(message string) (pass string, remember bool, ok bool)
	RememberPassphrase(keyID, pass string)
	LockVault()
	ExportData()
	ImportData()
	ExportSSHConfig()
	DragOut(paths []string)
	OpenURL(url string)

	Connect(h vault.Host)
	ConnectSplit(h vault.Host)
	OpenSFTP(h vault.Host)
	QuickConnect(target string) bool
	NewLocalTab(p shell.Profile)
	ImportSSHConfig()
	RunSnippet(s vault.Snippet, execute bool)
	ActiveSession() *session.Session

	Update() UpdateInfo
	CheckForUpdates()
	DownloadUpdate()
	RestartToUpdate()
}

type UpdateState uint8

const (
	UpdateIdle UpdateState = iota
	UpdateChecking
	UpdateCurrent
	UpdateAvailable
	UpdateDownloading
	UpdateReady
	UpdateFailed
)

type UpdateInfo struct {
	State    UpdateState
	Version  string
	Progress float32
	Err      string
}

func (i UpdateInfo) Label() string {
	switch i.State {
	case UpdateChecking:
		return "Checking for updates…"
	case UpdateCurrent:
		return "Boreal is up to date."
	case UpdateAvailable:
		return "Version " + i.Version + " is available."
	case UpdateDownloading:
		return "Downloading version " + i.Version + "… " + strconv.Itoa(int(i.Progress*100)) + "%"
	case UpdateReady:
		return "Version " + i.Version + " is ready. It installs when Boreal closes."
	case UpdateFailed:
		return "Couldn't update. " + i.Err
	}

	return ""
}
