package hosts

import (
	"image"
	"io"
	"slices"
	"sort"
	"strconv"
	"strings"

	"gioui.org/f32"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/app/core"
	"github.com/northlighthq/boreal/internal/app/dialogs"
	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/vault"
)

const (
	hostMime   = "application/x-boreal-host"
	folderMime = "application/x-boreal-folder"
)

const folderIndent = 16

const dragThreshold = 6

const (
	hostMenuConnect = iota
	hostMenuSplit
	hostMenuSFTP
	hostMenuSep1
	hostMenuEdit
	hostMenuDuplicate
	hostMenuMove
	hostMenuCopy
	hostMenuSep2
	hostMenuDelete
)

const (
	folderMenuNewHost = iota
	folderMenuNewFolder
	folderMenuRename
	folderMenuSep
	folderMenuDelete
)

const (
	moreNewHost = iota
	moreNewFolder
	moreImport
	moreQuick
)

type Panel struct {
	h core.Host

	sideList  kit.ScrollList
	add       widget.Clickable
	addFolder widget.Clickable
	more      widget.Clickable
	moreMenu  kit.Menu
	list      kit.ScrollList

	hosts   []vault.Host
	folders []string

	rows      map[string]*kit.Row
	drags     map[string]*widget.Draggable
	groups    map[string]*groupRow
	connect   map[string]*widget.Clickable
	collapsed map[string]bool
	selected  string

	menu       kit.Menu
	menuHost   vault.Host
	moveMenu   kit.Menu
	moveDest   []string
	folderMenu kit.Menu
	menuFolder string

	rootDrop bool
	dragging bool
	dragNow  bool
	over     map[*bool]bool
	moved    bool
	pressAt  map[*widget.Draggable]*f32.Point
	edit     folderEdit

	gen      int
	itemsKey itemsKey
	itemsOK  bool
	cached   []hostItem
}

type groupRow struct {
	row  kit.Row
	drag widget.Draggable
	drop bool
}

type itemsKey struct {
	gen    int
	edit   bool
	parent string
	path   string
}

type hostItem struct {
	folder string
	depth  int
	newDir bool
	full   bool
	host   *vault.Host
}

func (p *Panel) Title() string     { return "Hosts" }
func (p *Panel) Icon() *icons.Icon { return icons.FolderOutline }
func (p *Panel) Side() core.Side   { return core.Left }
func (p *Panel) Shortcut() string  { return "Alt+1" }

func (p *Panel) Refresh() {
	a := p.h
	d := a.Vault().Snapshot()
	p.hosts = d.Hosts
	p.folders = d.AllFolders()
	p.gen++
	sort.Slice(p.hosts, func(i, j int) bool {
		return strings.ToLower(p.hosts[i].Title()) < strings.ToLower(p.hosts[j].Title())
	})
}

func (p *Panel) ensure() {
	if p.rows != nil {
		return
	}

	p.rows = map[string]*kit.Row{}
	p.drags = map[string]*widget.Draggable{}
	p.groups = map[string]*groupRow{}
	p.connect = map[string]*widget.Clickable{}
	p.collapsed = map[string]bool{}
	p.over = map[*bool]bool{}
	p.pressAt = map[*widget.Draggable]*f32.Point{}
}

func (p *Panel) Actions(gtx C) D {
	a := p.h
	k := a.Kit()
	th := a.Kit().Th

	if p.add.Clicked(gtx) {
		a.OpenDocked(core.Right, NewEditor(a, Defaults()))
	}

	if p.addFolder.Clicked(gtx) {
		p.newFolder(a, "")
	}

	if p.more.Clicked(gtx) {
		p.moreMenu.Width = 240
		p.moreMenu.Open([]kit.MenuItem{
			moreNewHost:   {Label: "New host", Icon: icons.PlusOutline, Shortcut: "Ctrl+Shift+N"},
			moreNewFolder: {Label: "New folder…", Icon: icons.FolderOutline},
			moreImport:    {Label: "Import ~/.ssh/config", Icon: icons.DownloadOutline},
			moreQuick:     {Label: "Quick connect…", Icon: icons.ArrowRight},
		}, image.Pt(-gtx.Dp(200), gtx.Dp(28)))
	}

	if i, ok := p.moreMenu.Chosen(); ok {
		switch i {
		case moreNewHost:
			a.OpenDocked(core.Right, NewEditor(a, Defaults()))
		case moreNewFolder:
			p.newFolder(a, "")
		case moreImport:
			a.ImportSSHConfig()
		case moreQuick:
			a.ShowPalette()
		}
	}

	btn := func(clk *widget.Clickable, ic *icons.Icon) layout.FlexChild {
		return layout.Rigid(func(gtx C) D { return k.IconButtonSized(gtx, clk, ic, false, 26, 16, th.TextMuted) })
	}
	dims := layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		btn(&p.addFolder, icons.NewFolderOutline),
		btn(&p.add, icons.PlusOutline),
		btn(&p.more, icons.MoreH),
	)
	k.Menu(gtx, &p.moreMenu)
	return dims
}

func (p *Panel) items() []hostItem {
	key := itemsKey{p.gen, p.edit.active, p.edit.parent, p.edit.path}

	if p.itemsOK && key == p.itemsKey {
		return p.cached
	}

	p.cached, p.itemsKey, p.itemsOK = p.buildItems(), key, true
	return p.cached
}

func (p *Panel) buildItems() []hostItem {
	children := map[string][]string{}

	for _, f := range p.folders {
		parent := vault.FolderParent(f)
		children[parent] = append(children[parent], f)
	}

	hostsIn := map[string][]*vault.Host{}

	for i := range p.hosts {
		g := vault.CleanFolder(p.hosts[i].Group)
		hostsIn[g] = append(hostsIn[g], &p.hosts[i])
	}

	var out []hostItem
	var walk func(folder string, depth int)
	walk = func(folder string, depth int) {
		if p.edit.active && p.edit.path == "" && p.edit.parent == folder {
			out = append(out, hostItem{depth: depth, newDir: true})
		}

		for _, f := range children[folder] {
			out = append(out, hostItem{folder: f, depth: depth, full: len(children[f]) > 0 || len(hostsIn[f]) > 0})

			if !p.collapsed[f] {
				walk(f, depth+1)
			}
		}

		for _, h := range hostsIn[folder] {
			out = append(out, hostItem{depth: depth, host: h})
		}
	}
	walk("", 0)
	return out
}

func (p *Panel) Body(gtx C) D {
	a := p.h
	p.ensure()
	p.handleMenus(a, gtx)

	items := p.items()
	dims := a.Kit().List(gtx, &p.list, len(items), func(gtx C, i int) D {
		return p.item(a, gtx, items[i])
	})
	p.invalidateIfStale(gtx)
	return dims
}

func (p *Panel) invalidateIfStale(gtx C) {
	if (itemsKey{p.gen, p.edit.active, p.edit.parent, p.edit.path}) != p.itemsKey {
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (p *Panel) item(a core.Host, gtx C, it hostItem) D {
	if it.newDir {
		return p.newFolderRow(a, gtx, it.depth)
	}

	if it.host == nil {
		return p.groupHeader(a, gtx, it)
	}

	return p.hostRow(a, gtx, it.host, it.depth)
}

func (p *Panel) groupHeader(a core.Host, gtx C, it hostItem) D {
	k := a.Kit()
	th := a.Kit().Th

	g := p.groups[it.folder]

	if g == nil {
		g = &groupRow{drag: widget.Draggable{Type: folderMime}}
		p.groups[it.folder] = g
	}

	ev := g.row.Update(gtx)

	if ev.Click {
		p.collapsed[it.folder] = !p.collapsed[it.folder]
		p.gen++
	}

	if ev.Context {
		p.menuFolder = it.folder
		p.folderMenu.Width = 230
		p.folderMenu.Open([]kit.MenuItem{
			folderMenuNewHost:   {Label: "New host here", Icon: icons.PlusOutline},
			folderMenuNewFolder: {Label: "New subfolder…", Icon: icons.NewFolderOutline},
			folderMenuRename:    {Label: "Rename…", Icon: icons.Edit},
			folderMenuSep:       kit.Separator(),
			folderMenuDelete:    {Label: "Delete folder", Icon: icons.Delete, Danger: true},
		}, ev.At.Add(image.Pt(0, gtx.Dp(4))))
	}

	if mime, ok := g.drag.Update(gtx); ok {
		g.drag.Offer(gtx, mime, io.NopCloser(strings.NewReader(it.folder)))
	}

	p.acceptDrop(a, gtx, &g.drop, it.folder)

	hv := k.Hover(gtx, kit.Key(g, 9), g.row.Hovered())
	fg := kit.Lerp(th.TextMuted, th.Text, hv)
	row := func(gtx C) D {
		return k.Row(gtx, &g.row, kit.RowOpts{Indent: unit.Dp(it.depth * folderIndent)}, func(gtx C) D {
			chev := icons.ExpandMore

			if p.collapsed[it.folder] {
				chev = icons.ChevronRight
			}

			folder := icons.FolderOutline

			switch {
			case it.full && p.collapsed[it.folder]:
				folder = icons.FolderFilled
			case it.full:
				folder = icons.FolderOpen
			}

			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return chev.Layout(gtx, 16, th.TextFaint) }),
				layout.Rigid(kit.Gap(6)),
				layout.Rigid(func(gtx C) D { return folder.Layout(gtx, 18, fg) }),
				layout.Rigid(kit.Gap(10)),
				layout.Flexed(1, func(gtx C) D {
					if p.edit.active && p.edit.path == it.folder {
						return p.editField(a, gtx)
					}

					return k.Label(gtx, kit.TextStyle{Color: fg}, vault.FolderName(it.folder))
				}),
			)
		})
	}

	dims := p.draggable(a, gtx, &g.drag, row, vault.FolderName(it.folder), icons.FolderOutline)
	p.dropArea(a, gtx, &g.drop, dims.Size)

	if p.menuFolder == it.folder {
		k.Menu(gtx, &p.folderMenu)
	}

	return dims
}

func (p *Panel) hostRow(a core.Host, gtx C, h *vault.Host, depth int) D {
	k := a.Kit()
	th := a.Kit().Th

	r := p.rows[h.ID]

	if r == nil {
		r = &kit.Row{}
		p.rows[h.ID] = r
	}

	conn := p.connect[h.ID]

	if conn == nil {
		conn = &widget.Clickable{}
		p.connect[h.ID] = conn
	}

	drag := p.drags[h.ID]

	if drag == nil {
		drag = &widget.Draggable{Type: hostMime}
		p.drags[h.ID] = drag
	}

	if conn.Clicked(gtx) {
		a.Connect(*h)
	}

	ev := r.Update(gtx)

	if ev.Click {
		p.selected = h.ID
	}

	if ev.DoubleClick {
		a.Connect(*h)
	}

	if ev.Context {
		p.selected = h.ID
		p.menuHost = *h
		p.menu.Width = 250
		p.menu.Open([]kit.MenuItem{
			hostMenuConnect:   {Label: "Connect", Icon: icons.Play, Shortcut: "Double-click"},
			hostMenuSplit:     {Label: "Connect in split pane", Icon: icons.SplitRight},
			hostMenuSFTP:      {Label: "Open SFTP", Icon: icons.FolderOutline},
			hostMenuSep1:      kit.Separator(),
			hostMenuEdit:      {Label: "Edit…", Icon: icons.Edit},
			hostMenuDuplicate: {Label: "Duplicate", Icon: icons.Copy},
			hostMenuMove:      {Label: "Move to folder…", Icon: icons.FolderOutline},
			hostMenuCopy:      {Label: "Copy ssh command"},
			hostMenuSep2:      kit.Separator(),
			hostMenuDelete:    {Label: "Delete", Icon: icons.Delete, Danger: true},
		}, ev.At.Add(image.Pt(0, gtx.Dp(4))))
	}

	if mime, ok := drag.Update(gtx); ok {
		drag.Offer(gtx, mime, io.NopCloser(strings.NewReader(h.ID)))
	}

	selected := p.selected == h.ID
	indent := unit.Dp(depth*folderIndent + 22)

	if len(p.folders) == 0 {
		indent = 0
	}

	row := func(gtx C) D {
		hv := k.Hover(gtx, kit.Key(r, 9), r.Hovered())
		fg := kit.Lerp(kit.Lerp(th.TextMuted, th.Text, hv), th.Text, kit.B2F(selected))
		return k.Row(gtx, r, kit.RowOpts{Selected: selected, Focused: true, Indent: indent}, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return icons.Terminal.Layout(gtx, 16, kit.Lerp(th.TextMuted, fg, hv)) }),
				layout.Rigid(kit.Gap(10)),
				layout.Flexed(1, func(gtx C) D { return k.Label(gtx, kit.TextStyle{Color: fg}, h.Title()) }),
				layout.Rigid(func(gtx C) D {
					if !r.Hovered() && !conn.Hovered() {
						return D{}
					}

					return k.IconButtonSized(gtx, conn, icons.Play, false, 24, 16, th.TextMuted)
				}),
			)
		})
	}

	dims := p.draggable(a, gtx, drag, row, h.Title(), icons.Terminal)

	if p.menuHost.ID == h.ID {
		k.Menu(gtx, &p.menu)
		k.Menu(gtx, &p.moveMenu)
	}

	return dims
}

func (p *Panel) draggable(a core.Host, gtx C, d *widget.Draggable, row layout.Widget, label string, ic *icons.Icon) D {
	moved := d.Dragging() && d.Pos().X*d.Pos().X+d.Pos().Y*d.Pos().Y > float32(gtx.Dp(dragThreshold)*gtx.Dp(dragThreshold))

	if d.Dragging() {
		p.moved = moved
	}

	if moved {
		p.dragNow = true
	}

	at := p.pressAt[d]

	if at == nil {
		at = new(f32.Point)
		p.pressAt[d] = at
	}

	for {
		ev, ok := gtx.Event(pointer.Filter{Target: at, Kinds: pointer.Press})

		if !ok {
			break
		}

		if e, ok := ev.(pointer.Event); ok {
			*at = e.Position
		}
	}

	defer pointer.PassOp{}.Push(gtx.Ops).Pop()
	dims := d.Layout(gtx, row, func(gtx C) D {
		if !moved {
			return D{}
		}

		defer op.Offset(at.Round().Add(image.Pt(gtx.Dp(14), gtx.Dp(12)))).Push(gtx.Ops).Pop()
		return p.ghost(a, gtx, label, ic)
	})
	area := clip.Rect{Max: dims.Size}.Push(gtx.Ops)
	event.Op(gtx.Ops, at)
	area.Pop()
	return dims
}

func (p *Panel) ghost(a core.Host, gtx C, label string, ic *icons.Icon) D {
	k := a.Kit()
	th := a.Kit().Th
	gtx.Constraints.Min = image.Point{}
	return k.Surface(gtx, kit.SurfaceOpts{Radius: th.RadiusSmall, Fill: th.Popup, Shadow: true}, func(gtx C) D {
		return layout.Inset{Left: 8, Right: 10, Top: 4, Bottom: 4}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return ic.Layout(gtx, 14, th.TextMuted) }),
				layout.Rigid(kit.Gap(6)),
				layout.Rigid(func(gtx C) D { return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall}, label) }),
			)
		})
	})
}

func (p *Panel) dropArea(a core.Host, gtx C, tag *bool, size image.Point) {
	if p.dragging && p.over[tag] {
		th := a.Kit().Th
		r := image.Rect(gtx.Dp(6), 1, size.X-gtx.Dp(6), size.Y-1)
		kit.RRect(gtx, r, gtx.Dp(th.RadiusSmall), kit.Fade(th.Text, 0.06))
		kit.Border(gtx, r, gtx.Dp(th.RadiusSmall), 1, th.FieldLine)
	}

	defer pointer.PassOp{}.Push(gtx.Ops).Pop()
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, tag)
}

func (p *Panel) acceptDrop(a core.Host, gtx C, tag *bool, folder string) {
	for {
		ev, ok := gtx.Event(
			transfer.TargetFilter{Target: tag, Type: hostMime},
			transfer.TargetFilter{Target: tag, Type: folderMime},
			pointer.Filter{Target: tag, Kinds: pointer.Enter | pointer.Leave},
		)

		if !ok {
			return
		}

		switch e := ev.(type) {
		case pointer.Event:
			p.over[tag] = e.Kind == pointer.Enter
		case transfer.CancelEvent:
			p.over[tag] = false
		case transfer.DataEvent:
			p.over[tag] = false
			rc := e.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()

			if p.moved {
				p.moved = false
				p.drop(a, e.Type, string(b), folder)
			}
		}
	}
}

func (p *Panel) drop(a core.Host, mime, value, folder string) {
	switch mime {
	case hostMime:
		a.Vault().MoveHost(value, folder)
	case folderMime:
		if folder == value || vault.FolderParent(value) == folder {
			return
		}

		to := vault.CleanFolder(folder + vault.FolderSep + vault.FolderName(value))

		if slices.Contains(p.folders, to) {
			a.Toasts().Error("Folder already exists", "A folder named “"+vault.FolderName(to)+"” already exists there.")
			return
		}

		a.Vault().RenameFolder(value, to)
	}
}

func (p *Panel) handleMenus(a core.Host, gtx C) {
	if i, ok := p.menu.Chosen(); ok {
		p.hostAction(a, gtx, i)
	}

	if i, ok := p.moveMenu.Chosen(); ok && i < len(p.moveDest) {
		a.Vault().MoveHost(p.menuHost.ID, p.moveDest[i])
	}

	if i, ok := p.folderMenu.Chosen(); ok {
		p.folderAction(a, i)
	}
}

func (p *Panel) hostAction(a core.Host, gtx C, i int) {
	h := p.menuHost

	switch i {
	case hostMenuConnect:
		a.Connect(h)
	case hostMenuSplit:
		a.ConnectSplit(h)
	case hostMenuSFTP:
		a.OpenSFTP(h)
	case hostMenuEdit:
		a.OpenDocked(core.Right, NewEditor(a, h))
	case hostMenuDuplicate:
		h.ID = ""
		h.Label += " (copy)"
		a.Vault().SaveHost(h)
	case hostMenuMove:
		p.openMoveMenu(gtx, h)
	case hostMenuCopy:
		cmd := "ssh " + h.Target()

		if h.Port != 22 && h.Port != 0 {
			cmd = "ssh -p " + strconv.Itoa(h.Port) + " " + strings.TrimSuffix(h.Target(), ":"+strconv.Itoa(h.Port))
		}

		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(cmd))})
		a.Toasts().Info("Copied", cmd)
	case hostMenuDelete:
		a.Open(dialogs.NewConfirm(a.Kit(), "Delete this host?", "“"+h.Title()+"” and its port forwarding rules will be removed.", "Delete", func() {
			a.Vault().DeleteHost(h.ID)
		}))
	}
}

func (p *Panel) openMoveMenu(gtx C, h vault.Host) {
	current := vault.CleanFolder(h.Group)
	p.moveDest = append(p.moveDest[:0], "")
	items := []kit.MenuItem{{Label: "No folder", Icon: icons.HomeOutline, Disabled: current == ""}}

	for _, f := range p.folders {
		p.moveDest = append(p.moveDest, f)
		label := strings.Repeat("    ", strings.Count(f, vault.FolderSep)) + vault.FolderName(f)
		items = append(items, kit.MenuItem{Label: label, Icon: icons.FolderOutline, Disabled: f == current})
	}

	p.moveMenu.Width = 240
	p.moveMenu.Open(items, image.Pt(gtx.Dp(24), gtx.Dp(30)))
}

func (p *Panel) folderAction(a core.Host, i int) {
	path := p.menuFolder

	switch i {
	case folderMenuNewHost:
		h := Defaults()
		h.Group = path
		a.OpenDocked(core.Right, NewEditor(a, h))
	case folderMenuNewFolder:
		p.newFolder(a, path)
	case folderMenuRename:
		p.renameFolder(a, path)
	case folderMenuDelete:
		dest := "the root"

		if parent := vault.FolderParent(path); parent != "" {
			dest = "“" + vault.FolderName(parent) + "”"
		}

		msg := "“" + vault.FolderName(path) + "” and its subfolders will be removed. Their hosts move to " + dest + "."
		a.Open(dialogs.NewConfirm(a.Kit(), "Delete this folder?", msg, "Delete", func() {
			a.Vault().DeleteFolder(path)
		}))
	}
}

func (p *Panel) beginFrame() {
	p.dragging = p.dragNow
	p.dragNow = false
}

type sideItem struct {
	header string
	host   hostItem
	empty  bool
}

func (p *Panel) Sidebar(gtx C) D {
	a := p.h
	k := a.Kit()
	p.ensure()
	p.beginFrame()
	p.handleMenus(a, gtx)

	var items []sideItem
	items = append(items, sideItem{header: "Hosts"})
	hosts := p.items()

	for _, it := range hosts {
		items = append(items, sideItem{host: it})
	}

	if len(hosts) == 0 {
		items = append(items, sideItem{empty: true})
	}

	dims := layout.Inset{Left: 8, Right: 8}.Layout(gtx, func(gtx C) D {
		return k.List(gtx, &p.sideList, len(items), func(gtx C, i int) D {
			it := items[i]

			switch {
			case it.header != "":
				return p.sectionHeader(a, gtx, it.header, it.header == "Hosts")
			case it.empty:
				return p.hostsEmpty(a, gtx)
			default:
				return p.item(a, gtx, it.host)
			}
		})
	})
	p.invalidateIfStale(gtx)
	return dims
}

func (p *Panel) sectionHeader(a core.Host, gtx C, title string, hosts bool) D {
	k := a.Kit()
	th := a.Kit().Th

	if hosts {
		p.acceptDrop(a, gtx, &p.rootDrop, "")
	}

	dims := layout.Inset{Left: 12, Right: 4, Top: 18, Bottom: 4}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.Y = gtx.Dp(26)
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextFaint}, title)
			}),
			layout.Rigid(func(gtx C) D {
				if !hosts {
					return D{}
				}

				return p.Actions(gtx)
			}),
		)
	})

	if hosts {
		p.dropArea(a, gtx, &p.rootDrop, dims.Size)
	}

	return dims
}

func (p *Panel) hostsEmpty(a core.Host, gtx C) D {
	k := a.Kit()
	th := a.Kit().Th
	return layout.Inset{Left: 12, Top: 4, Bottom: 8}.Layout(gtx, func(gtx C) D {
		return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextFaint}, "No saved hosts yet")
	})
}

func New(h core.Host) *Panel {
	return &Panel{h: h}
}

type (
	C = core.C
	D = core.D
)
