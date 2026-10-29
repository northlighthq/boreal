package hosts

import (
	"slices"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/northlighthq/boreal/internal/app/core"
	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/vault"
)

type folderEdit struct {
	active bool
	parent string
	path   string
	field  kit.InlineEdit
}

func (p *Panel) newFolder(a core.Host, parent string) {
	p.startEdit(a, parent, "", "")
	p.collapsed[parent] = false
	p.gen++
}

func (p *Panel) renameFolder(a core.Host, path string) {
	p.startEdit(a, vault.FolderParent(path), path, vault.FolderName(path))
}

func (p *Panel) startEdit(a core.Host, parent, path, name string) {
	a.CloseDocked(core.Left)
	a.ShowTool(p)

	p.edit = folderEdit{active: true, parent: parent, path: path}
	p.edit.field.Start(a.Kit(), name)
}

func (p *Panel) commitEdit(a core.Host) {
	e := &p.edit
	name := vault.CleanFolder(e.field.Text())
	e.active = false

	if name == "" || strings.Contains(name, vault.FolderSep) && e.path != "" {
		return
	}

	to := name

	if e.parent != "" {
		to = e.parent + vault.FolderSep + name
	}

	if to != e.path && slices.Contains(p.folders, to) {
		if e.field.Blurred() {
			return
		}

		a.Toasts().Error("Folder already exists", "A folder named “"+vault.FolderName(to)+"” already exists here.")
		e.active = true
		e.field.Start(a.Kit(), e.field.Text())
		return
	}

	if e.path == "" {
		a.Vault().CreateFolder(to)
		return
	}

	if to != e.path {
		a.Vault().RenameFolder(e.path, to)
	}
}

func (p *Panel) editField(a core.Host, gtx C) D {
	dims, res := p.edit.field.Layout(gtx, a.Kit(), "Folder name")

	switch res {
	case kit.EditCommit:
		p.commitEdit(a)
	case kit.EditCancel:
		p.edit.active = false
	}

	return dims
}

func (p *Panel) newFolderRow(a core.Host, gtx C, depth int) D {
	th := a.Kit().Th
	h := gtx.Dp(a.Kit().Th.RowHeight)
	return layout.Inset{Left: unit.Dp(depth*folderIndent + 12 + 22), Right: 12}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.Y = h
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return icons.FolderOutline.Layout(gtx, 18, th.Text) }),
			layout.Rigid(kit.Gap(8)),
			layout.Flexed(1, func(gtx C) D { return p.editField(a, gtx) }),
		)
	})
}
