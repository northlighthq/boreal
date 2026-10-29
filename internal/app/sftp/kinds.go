package sftp

import (
	"image/color"
	"os"
	"path"
	"strings"

	"github.com/northlighthq/boreal/internal/sshx"
	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/theme"
)

const (
	tagBlue = iota
	tagGreen
	tagYellow
	tagRed
	tagPurple
	tagCyan
	tagOrange
	tagPink
)

var kindByExt = map[string]int{}

func init() {
	groups := map[int]string{
		tagBlue:   "go py js mjs ts tsx jsx rs c h cc cpp hpp java kt rb php lua swift cs scala dart vue svelte sql html css scss",
		tagYellow: "json yaml yml toml ini conf cfg env xml properties service timer",
		tagOrange: "pem key crt cer csr pub gpg asc p12 pfx",
		tagRed:    "zip tar gz tgz xz bz2 7z rar zst deb rpm apk jar iso img",
		tagPurple: "png jpg jpeg gif svg webp ico bmp heic avif mp4 mkv mov avi webm mp3 wav flac ogg m4a",
		tagPink:   "md txt rst pdf doc docx csv",
	}

	for tag, exts := range groups {
		for _, e := range strings.Fields(exts) {
			kindByExt[e] = tag
		}
	}
}

func entryStyle(th *theme.Theme, e sshx.Entry) (*icons.Icon, color.NRGBA) {
	switch {
	case e.IsDir && !th.Colorful():
		return icons.Folder, th.TextMuted
	case e.IsDir:
		return icons.Folder, th.Accent
	}

	ext := strings.ToLower(strings.TrimPrefix(path.Ext(e.Name), "."))
	ic := icons.File
	tag := -1

	switch {
	case e.Mode&os.ModeSymlink != 0:
		ic, tag = icons.Link, tagCyan
	case ext == "sh" || ext == "bash" || ext == "zsh" || ext == "fish":
		ic, tag = icons.Terminal, tagGreen
	case strings.HasPrefix(e.Name, "id_") || e.Name == "authorized_keys" || e.Name == "known_hosts":
		ic, tag = icons.Key, tagOrange
	case known(ext):
		tag = kindByExt[ext]
	case e.Mode.Perm()&0o111 != 0:
		ic, tag = icons.Terminal, tagGreen
	}

	switch tag {
	case tagBlue:
		ic = icons.CodeOutline
	case tagYellow:
		ic = icons.Settings
	case tagOrange:
		ic = icons.Key
	case tagRed:
		ic = icons.FileArchive
	case tagPink:
		ic = icons.FileText
	case tagPurple:
		ic = icons.FileImage

		if strings.Contains(" mp3 wav flac ogg m4a ", " "+ext+" ") {
			ic = icons.FileAudio
		}
	}

	if ext == "log" {
		ic = icons.FileText
	}

	if tag < 0 || !th.Colorful() || ext == "log" {
		return ic, th.TextFaint
	}

	return ic, th.TagColor(tag)
}

func known(ext string) bool {
	_, ok := kindByExt[ext]
	return ok
}
