package hosts

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/common/icons"
	"github.com/northlighthq/boreal/internal/common/kit"
	"github.com/northlighthq/boreal/internal/core/sshx"
)

const (
	anyAgentKey    = "Any key in the agent"
	fingerprintLen = 16
)

type agentField struct {
	path    widget.Editor
	refresh widget.Clickable
	key     kit.Select
	keys    []string
	opts    []kit.Option
	status  string
	failed  bool
	remote  bool
}

func (f *agentField) load(selected string) {
	path := strings.TrimSpace(f.path.Text())
	info, err := sshx.InspectAgent(path)
	f.remote = !sshx.LocalAgentPath(path)
	f.failed = err != nil
	f.status = agentStatus(info, err)
	f.keys = []string{""}
	f.opts = []kit.Option{{Label: anyAgentKey}}

	for _, k := range info.Keys {
		f.keys = append(f.keys, k.Fingerprint)
		f.opts = append(f.opts, kit.Option{Label: agentKeyLabel(k), Icon: icons.Key})
	}

	if selected != "" && !slices.Contains(f.keys, selected) {
		f.keys = append(f.keys, selected)
		f.opts = append(f.opts, kit.Option{Label: shortFingerprint(selected) + "  ·  not in the agent now", Icon: icons.Key})
	}

	f.key.Value = max(slices.Index(f.keys, selected), 0)
}

func (f *agentField) chosen() string {
	if f.key.Value < 0 || f.key.Value >= len(f.keys) {
		return ""
	}

	return f.keys[f.key.Value]
}

func agentStatus(info sshx.AgentInfo, err error) string {
	switch {
	case errors.Is(err, sshx.ErrAgentOff):
		return "The agent is off for this host."
	case errors.Is(err, sshx.ErrRemoteAgent):
		return "Only an agent on this computer can be used."
	case err != nil && info.Path != "":
		return "Found " + sshx.AgentName(info.Path) + ", but it didn't list its keys. It may be locked."
	case err != nil:
		return "No agent found."
	case len(info.Keys) == 1:
		return "Found " + sshx.AgentName(info.Path) + " with 1 key."
	}

	return "Found " + sshx.AgentName(info.Path) + " with " + strconv.Itoa(len(info.Keys)) + " keys."
}

func agentKeyLabel(k sshx.AgentKey) string {
	name := k.Comment

	if name == "" {
		name = "Unnamed key"
	}

	return name + "  ·  " + strings.ToUpper(strings.TrimPrefix(k.Type, "ssh-")) + "  ·  " + shortFingerprint(k.Fingerprint)
}

func shortFingerprint(fp string) string {
	fp = strings.TrimPrefix(fp, "SHA256:")

	if len(fp) > fingerprintLen {
		fp = fp[:fingerprintLen] + "…"
	}

	return fp
}

func (e *Editor) agentRows(field func(string, layout.Widget) layout.FlexChild) []layout.FlexChild {
	k := e.h.Kit()
	th := k.Th
	f := &e.agent

	return []layout.FlexChild{
		field("Agent", func(gtx C) D {
			if f.refresh.Clicked(gtx) {
				f.load(f.chosen())
			}

			color := th.TextMuted

			if f.failed {
				color = th.Danger
			}

			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return k.Field(gtx, &f.path, kit.FieldOpts{
						Hint:    "Automatic",
						Mono:    true,
						Invalid: f.remote,
						Trailing: func(gtx C) D {
							return k.IconButtonSized(gtx, &f.refresh, icons.Refresh, false, 22, 16, th.TextMuted)
						},
					})
				}),
				layout.Rigid(kit.Gap(6)),
				layout.Rigid(func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: color, MaxLines: 2}, f.status)
				}),
				layout.Rigid(kit.Gap(4)),
				layout.Rigid(func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted, MaxLines: 3}, agentHint)
				}),
			)
		}),
		field("Key", func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return k.SelectOptions(gtx, &f.key, f.opts) }),
				layout.Rigid(kit.Gap(4)),
				layout.Rigid(func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted, MaxLines: 3}, "With many keys in the agent, pick the one for this host. Servers usually stop after 6 tries.")
				}),
			)
		}),
	}
}
