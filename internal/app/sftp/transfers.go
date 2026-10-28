package sftp

import (
	"fmt"
	"image"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/app/core"
	"github.com/northlighthq/boreal/internal/format"
	"github.com/northlighthq/boreal/internal/platform/open"
	"github.com/northlighthq/boreal/internal/transfer"
	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/ui/theme"
)

const (
	editPoll    = time.Second
	maxJobsShow = 6
)

type jobRow struct {
	cancel widget.Clickable
}

func (p *Panel) enqueue(a core.Host, c *sftpConn, j *transfer.Job, reload string, after func(*transfer.Job)) {
	client := c.sftp()

	if client == nil {
		a.Toasts().Error("SFTP unavailable", "The session isn't connected.")
		return
	}

	a.Transfers().Add(client.Client, j, func(j *transfer.Job) {
		switch j.State() {
		case transfer.Done:
			done, _ := j.Progress()
			verb := "Downloaded "

			if j.Kind == transfer.Upload {
				verb = "Uploaded "
			}

			if after == nil {
				a.Toasts().Success(verb+j.Name, size(done, j.Files()))
			}
		case transfer.Failed:
			if j.Local == "" {
				a.Toasts().Error("Transfer failed: "+j.Name, j.Err().Error())
				break
			}

			retry := &transfer.Job{Kind: j.Kind, Name: j.Name, Local: j.Local, Remote: j.Remote}
			a.Toasts().ErrorAction("Transfer failed: "+j.Name, j.Err().Error(), "Retry", func() {
				p.enqueue(a, c, retry, reload, after)
				a.Invalidate()
			})
		}

		if reload != "" && j.State() == transfer.Done {
			p.load(a, c, reload)
		}

		if after != nil && j.State() == transfer.Done {
			after(j)
		}
	})
}

func size(n, files int64) string {
	if files > 1 {
		return fmt.Sprintf("%s in %d files", format.Size(n), files)
	}

	return format.Size(n)
}

func (p *Panel) uploadPath(a core.Host, c *sftpConn, local, cwd string) {
	name := filepath.Base(local)
	p.enqueue(a, c, &transfer.Job{Kind: transfer.Upload, Name: name, Local: local, Remote: path.Join(cwd, name)}, cwd, nil)
}

func (p *Panel) UploadDropped(sessID string, paths []string) {
	a := p.h
	var c *sftpConn

	for range 100 {
		c = p.connFor(sessID)

		if c != nil {
			if cwd, _, loading, _ := c.snapshot(); cwd != "" && !loading {
				break
			}
		}

		a.Invalidate()
		time.Sleep(50 * time.Millisecond)
	}

	if c == nil || c.sftp() == nil {
		a.Toasts().Error("SFTP unavailable", "Couldn't open SFTP in this session.")
		return
	}

	cwd, _, _, _ := c.snapshot()

	for _, local := range paths {
		p.uploadPath(a, c, local, cwd)
	}
}

func (p *Panel) uploadFile(a core.Host, c *sftpConn, cwd string) {
	f, err := a.Explorer().ChooseFile()

	if err != nil {
		return
	}

	named, ok := f.(*os.File)
	f.Close()

	if !ok {
		a.Toasts().Error("Couldn't upload", "The file picker didn't return a file on disk.")
		return
	}

	p.uploadPath(a, c, named.Name(), cwd)
}

func (p *Panel) downloadFile(a core.Host, c *sftpConn, src string) {
	name := transfer.SafeName(path.Base(src))
	w, err := a.Explorer().CreateFile(name)

	if err != nil {
		return
	}

	p.enqueue(a, c, &transfer.Job{Kind: transfer.Download, Name: name, Remote: src, Writer: w}, "", nil)
}

func (p *Panel) downloadFolder(a core.Host, c *sftpConn, src string) {
	home, err := os.UserHomeDir()

	if err != nil {
		a.Toasts().Error("Couldn't download", err.Error())
		return
	}

	name := transfer.SafeName(path.Base(src))
	dst := unique(filepath.Join(home, "Downloads", name))

	p.enqueue(a, c, &transfer.Job{Kind: transfer.Download, Name: name, Remote: src, Local: dst}, "", func(j *transfer.Job) {
		done, _ := j.Progress()
		a.Toasts().SuccessAction("Downloaded "+name, size(done, j.Files())+" · saved in Downloads", "Open folder", func() { open.File(dst) })
	})
}

func unique(p string) string {
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return p
	}

	ext := filepath.Ext(p)
	base := strings.TrimSuffix(p, ext)

	for i := 2; ; i++ {
		c := fmt.Sprintf("%s (%d)%s", base, i, ext)

		if _, err := os.Stat(c); os.IsNotExist(err) {
			return c
		}
	}
}

func (p *Panel) edit(a core.Host, c *sftpConn, src, cwd string) {
	dir, err := os.MkdirTemp("", "boreal-edit-")

	if err != nil {
		a.Toasts().Error("Couldn't edit", err.Error())
		return
	}

	name := transfer.SafeName(path.Base(src))
	local := filepath.Join(dir, name)

	p.enqueue(a, c, &transfer.Job{Kind: transfer.Download, Name: name, Remote: src, Local: local}, "", func(*transfer.Job) {
		info, err := os.Stat(local)

		if err != nil {
			return
		}

		if err := open.Edit(local); err != nil {
			a.Toasts().Error("Couldn't open "+name, err.Error())
			return
		}

		a.Toasts().Info("Editing "+name, "Save it in your editor and Boreal uploads it back.")
		go p.watch(a, c, local, src, cwd, info.ModTime())
	})
}

func (p *Panel) watch(a core.Host, c *sftpConn, local, remote, cwd string, last time.Time) {
	defer os.RemoveAll(filepath.Dir(local))

	closed := make(chan struct{})

	go func() {
		c.owner.Wait()
		close(closed)
	}()

	tick := time.NewTicker(editPoll)
	defer tick.Stop()

	for {
		select {
		case <-closed:
			return
		case <-tick.C:
		}

		info, err := os.Stat(local)

		if err != nil {
			return
		}

		if !info.ModTime().After(last) {
			continue
		}

		last = info.ModTime()
		name := path.Base(remote)
		p.enqueue(a, c, &transfer.Job{Kind: transfer.Upload, Name: name, Local: local, Remote: remote}, cwd, func(*transfer.Job) {
			a.Toasts().Success("Saved "+name, "Uploaded your changes to the server.")
		})
	}
}

func (p *Panel) transfers(a core.Host, gtx C) D {
	k := a.Kit()
	th := k.Th
	q := a.Transfers()
	jobs := q.Jobs()

	if len(jobs) == 0 {
		return D{}
	}

	if len(jobs) > maxJobsShow {
		jobs = jobs[len(jobs)-maxJobsShow:]
	}

	if p.jobRows == nil {
		p.jobRows = map[int]*jobRow{}
	}

	if p.clear.Clicked(gtx) {
		q.ClearFinished()
	}

	rows := []layout.FlexChild{
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: 4, Right: 4, Bottom: 6}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D {
						title := "Transfers"

						if n := q.Active(); n > 0 {
							title = fmt.Sprintf("Transfers · %d active", n)
						}

						return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextFaint}, title)
					}),
					layout.Rigid(func(gtx C) D {
						if q.Active() == len(q.Jobs()) {
							return D{}
						}

						return k.Button(gtx, &p.clear, kit.Ghost, nil, "Clear")
					}),
				)
			})
		}),
	}

	for i := len(jobs) - 1; i >= 0; i-- {
		j := jobs[i]
		r := p.jobRows[j.ID]

		if r == nil {
			r = &jobRow{}
			p.jobRows[j.ID] = r
		}

		if r.cancel.Clicked(gtx) {
			q.Cancel(j.ID)
		}

		rows = append(rows, layout.Rigid(func(gtx C) D { return p.jobRow(a, gtx, j, r) }))
	}

	return layout.Inset{Left: 8, Right: 8, Top: 8, Bottom: 4}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
	})
}

func (p *Panel) jobRow(a core.Host, gtx C, j *transfer.Job, r *jobRow) D {
	k := a.Kit()
	th := k.Th
	done, total := j.Progress()
	ic := icons.Upload

	if j.Kind == transfer.Download {
		ic = icons.Download
	}

	status := format.Size(done) + " of " + format.Size(total)
	color := th.TextMuted

	switch j.State() {
	case transfer.Queued:
		status = "Waiting…"
	case transfer.Done:
		status = size(done, j.Files())
	case transfer.Failed:
		status, color = j.Err().Error(), th.Danger
	case transfer.Cancelled:
		status = "Cancelled"
	}

	frac := float32(1)

	if total > 0 {
		frac = float32(done) / float32(total)
	}

	return layout.Inset{Left: 4, Right: 4, Bottom: 8}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return ic.Layout(gtx, 16, th.TextMuted) }),
			layout.Rigid(kit.Gap(10)),
			layout.Flexed(1, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return k.Label(gtx, kit.TextStyle{Weight: font.Medium, MaxLines: 1}, j.Name)
					}),
					layout.Rigid(kit.Gap(4)),
					layout.Rigid(func(gtx C) D {
						if j.State() != transfer.Running {
							return D{}
						}

						return layout.Inset{Bottom: 4}.Layout(gtx, func(gtx C) D { return bar(gtx, th, frac) })
					}),
					layout.Rigid(func(gtx C) D {
						return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall - 1, Color: color, MaxLines: 1}, status)
					}),
				)
			}),
			layout.Rigid(func(gtx C) D {
				if j.Finished() {
					return D{}
				}

				return k.IconButtonSized(gtx, &r.cancel, icons.Close, false, 24, 14, th.TextMuted)
			}),
		)
	})
}

func bar(gtx C, th *theme.Theme, frac float32) D {
	w, h := gtx.Constraints.Max.X, gtx.Dp(4)
	kit.RRect(gtx, image.Rect(0, 0, w, h), h/2, kit.Fade(th.Text, 0.12))

	fill := th.TextMuted

	if th.Colorful() {
		fill = th.Accent
	}

	if fw := int(float32(w) * min(max(frac, 0), 1)); fw > 0 {
		kit.RRect(gtx, image.Rect(0, 0, max(fw, h), h), h/2, fill)
	}

	return D{Size: image.Pt(w, h)}
}

func (p *Panel) downloadEntry(a core.Host, c *sftpConn, cwd, name string) {
	full := path.Join(cwd, name)

	_, entries, _, _ := c.snapshot()

	for _, e := range entries {
		if e.Name == name && e.IsDir {
			p.downloadFolder(a, c, full)
			return
		}
	}

	go p.downloadFile(a, c, full)
}

func (p *Panel) Ready(sessID string) bool {
	c := p.connFor(sessID)

	if c == nil {
		return false
	}

	cwd, _, loading, _ := c.snapshot()
	return c.sftp() != nil && cwd != "" && !loading
}
