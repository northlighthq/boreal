package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	appDir       = "Platty"
	settingsFile = "settings.json"
)

const (
	CursorBlock     = "block"
	CursorBar       = "bar"
	CursorUnderline = "underline"
)

const (
	ClipboardAsk   = "ask"
	ClipboardAllow = "allow"
	ClipboardBlock = "block"
)

type Settings struct {
	Theme         string  `json:"theme"`
	FontSize      float32 `json:"fontSize"`
	LineHeight    float32 `json:"lineHeight"`
	CursorStyle   string  `json:"cursorStyle"`
	CursorBlink   bool    `json:"cursorBlink"`
	Scrollback    int     `json:"scrollback"`
	CopyOnSelect  bool    `json:"copyOnSelect"`
	Shell         string  `json:"shell"`
	ConfirmClose  bool    `json:"confirmClose"`
	HideAddresses bool    `json:"hideAddresses"`
	TerminalFont  string  `json:"terminalFont"`
	KeepAlive     int     `json:"keepAliveSeconds"`
	LockAfter     int     `json:"lockAfterMinutes"`
	SidebarWidth  float32 `json:"sidebarWidth"`
	SidebarOpen   string  `json:"sidebarOpen"`
	DesignVersion int     `json:"designVersion"`
	Onboarded     bool    `json:"onboarded"`
	RestoreTabs   bool    `json:"restoreTabs"`
	Ligatures     bool    `json:"ligatures"`
	Highlight     bool    `json:"highlightOutput"`
	Clipboard     string  `json:"serverClipboard"`
	AutoUpdate    bool    `json:"autoUpdate"`
	BroadcastSkip string  `json:"broadcastSkipTags"`
	Accent        string  `json:"accent"`
	Density       string  `json:"density"`
	Corners       string  `json:"corners"`
	TermScheme    string  `json:"terminalScheme"`
	TabStyle      string  `json:"tabStyle"`
	TabIcons      bool    `json:"tabIcons"`
}

const designVersion = 3

var themes = map[string]struct{}{"carbon": {}, "graphite": {}, "midnight": {}, "paper": {}, "catppuccin": {}, "rosepine": {}, "nord": {}, "everforest": {}, "gruvbox": {}, "tokyonight": {}, "latte": {}, "dawn": {}}

var renamedThemes = map[string]string{"ember": "gruvbox", "tide": "tokyonight", "moss": "everforest", "dusk": "catppuccin", "bloom": "latte"}

func Defaults() Settings {
	return Settings{
		Theme:         "northlight",
		FontSize:      13,
		LineHeight:    1.2,
		CursorStyle:   CursorBlock,
		CursorBlink:   true,
		Scrollback:    10_000,
		CopyOnSelect:  false,
		ConfirmClose:  true,
		KeepAlive:     30,
		LockAfter:     15,
		RestoreTabs:   true,
		Clipboard:     ClipboardAsk,
		AutoUpdate:    true,
		Ligatures:     true,
		Highlight:     true,
		BroadcastSkip: "prod, production",
		SidebarWidth:  280,
		SidebarOpen:   "hosts",
		Density:       "normal",
		Corners:       "normal",
		TabStyle:      "classic",
		TabIcons:      true,
		DesignVersion: designVersion,
	}
}

func Dir() (string, error) {
	base, err := os.UserConfigDir()

	if err != nil {
		return "", err
	}

	dir := filepath.Join(base, appDir)

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}

	return dir, nil
}

func Load() Settings {
	s := Defaults()
	dir, err := Dir()

	if err != nil {
		return s
	}

	b, err := os.ReadFile(filepath.Join(dir, settingsFile))

	if err != nil {
		return s
	}

	json.Unmarshal(b, &s)
	return s.sanitized()
}

func Save(s Settings) error {
	dir, err := Dir()

	if err != nil {
		return err
	}

	b, err := json.MarshalIndent(s, "", "  ")

	if err != nil {
		return err
	}

	return writeAtomic(filepath.Join(dir, settingsFile), b)
}

func (s Settings) sanitized() Settings {
	d := Defaults()

	if s.DesignVersion < designVersion {
		if s.Theme == "light" {
			s.Theme = "paper"
		} else {
			s.Theme = d.Theme
		}

		s.DesignVersion = designVersion
	}

	if n, ok := renamedThemes[s.Theme]; ok {
		s.Theme = n
	}

	if _, ok := themes[s.Theme]; !ok {
		s.Theme = d.Theme
	}

	if s.FontSize < 8 || s.FontSize > 40 {
		s.FontSize = d.FontSize
	}

	if s.LineHeight < 1 || s.LineHeight > 2 {
		s.LineHeight = d.LineHeight
	}

	if s.Scrollback < 0 {
		s.Scrollback = d.Scrollback
	}

	if s.SidebarWidth < 200 {
		s.SidebarWidth = d.SidebarWidth
	}

	if s.Clipboard != ClipboardAllow && s.Clipboard != ClipboardBlock {
		s.Clipboard = ClipboardAsk
	}

	return s
}

func writeAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)

	if err != nil {
		return err
	}

	_, err = f.Write(data)

	if err == nil {
		err = f.Sync()
	}

	if cerr := f.Close(); err == nil {
		err = cerr
	}

	if err != nil {
		os.Remove(tmp)
		return err
	}

	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}

	return nil
}

func WriteAtomic(path string, data []byte) error {
	return writeAtomic(path, data)
}
