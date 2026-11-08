package settings

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/common/icons"
	"github.com/northlighthq/boreal/internal/common/kit"
	"github.com/northlighthq/boreal/internal/common/mark"
	"github.com/northlighthq/boreal/internal/core/version"
	"github.com/northlighthq/boreal/internal/desktop/base"
)

type about struct {
	website, repo, issue, licenses widget.Clickable
	upgrade                        widget.Clickable
}

func (s *about) update(a base.Host, gtx C) {
	links := map[*widget.Clickable]string{
		&s.website:  version.Website,
		&s.repo:     version.Repo,
		&s.issue:    version.Repo + "/issues/new",
		&s.licenses: version.Repo + "/blob/main/THIRD_PARTY_NOTICES.md",
	}

	for clk, url := range links {
		if clk.Clicked(gtx) {
			a.OpenURL(url)
		}
	}

	if !s.upgrade.Clicked(gtx) {
		return
	}

	switch info := a.Update(); {
	case info.State == base.UpdateReady:
		a.RestartToUpdate()
	case info.State == base.UpdateAvailable, info.State == base.UpdateFailed && info.Version != "":
		a.DownloadUpdate()
	case info.State != base.UpdateChecking && info.State != base.UpdateDownloading:
		a.CheckForUpdates()
	}
}

func updateAction(info base.UpdateInfo) (*icons.Icon, string) {
	switch info.State {
	case base.UpdateChecking, base.UpdateDownloading:
		return nil, ""
	case base.UpdateAvailable:
		return icons.Download, "Download update"
	case base.UpdateReady:
		return icons.Refresh, "Restart now"
	case base.UpdateFailed:
		if info.Version != "" {
			return icons.Download, "Try again"
		}
	}

	return icons.Refresh, "Check for updates"
}

func (d *Dialog) updates(a base.Host) []layout.FlexChild {
	k := a.Kit()
	th := k.Th
	info := a.Update()
	ic, action := updateAction(info)

	return []layout.FlexChild{
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					if action == "" {
						return D{}
					}

					return layout.Inset{Right: 12}.Layout(gtx, func(gtx C) D {
						return k.Button(gtx, &d.about.upgrade, kit.Secondary, ic, action)
					})
				}),
				layout.Flexed(1, func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Color: th.TextMuted, MaxLines: 2}, info.Label())
				}),
			)
		}),
		layout.Rigid(kit.Gap(12)),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return k.Toggle(gtx, &d.autoUpdate) }),
				layout.Rigid(kit.Gap(12)),
				layout.Rigid(func(gtx C) D {
					return k.Label(gtx, kit.TextStyle{Color: th.TextMuted}, "Check for updates automatically")
				}),
			)
		}),
	}
}

func (d *Dialog) aboutPage(a base.Host, gtx C) []layout.FlexChild {
	k := a.Kit()
	th := k.Th
	s := &d.about
	s.update(a, gtx)

	build := "Version " + version.Version

	if c := version.Commit(); c != "" {
		build += " · " + c
	}

	build += " · " + version.Go()

	button := func(clk *widget.Clickable, ic *icons.Icon, label string) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			return layout.Inset{Right: 8}.Layout(gtx, func(gtx C) D { return k.Button(gtx, clk, kit.Secondary, ic, label) })
		})
	}

	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return mark.Layout(gtx, 40) }),
				layout.Rigid(kit.Gap(16)),
				layout.Rigid(func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return k.Label(gtx, kit.TextStyle{Size: th.TextSizeTitle, Weight: font.SemiBold}, "Boreal")
						}),
						layout.Rigid(kit.Gap(4)),
						layout.Rigid(func(gtx C) D {
							return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted, Mono: true}, build)
						}),
					)
				}),
			)
		}),
		layout.Rigid(kit.Gap(18)),
	}

	children = append(children, d.updates(a)...)
	children = append(children,
		layout.Rigid(kit.Gap(22)),
		layout.Rigid(func(gtx C) D {
			return k.Label(gtx, kit.TextStyle{Color: th.TextMuted, MaxLines: 2}, "A fast, native SSH client and terminal.")
		}),
		layout.Rigid(kit.Gap(18)),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				button(&s.website, icons.Launch, "Website"),
				button(&s.repo, icons.CodeOutline, "GitHub"),
				button(&s.issue, icons.Error, "Report a problem"),
				button(&s.licenses, icons.File, "Licenses"),
			)
		}),
		layout.Rigid(kit.Gap(22)),
		layout.Rigid(func(gtx C) D {
			return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextFaint}, "© 2026 AlesixDev · MIT License · Icons by Phosphor · Fonts under the SIL Open Font License")
		}),
	)

	return children
}
