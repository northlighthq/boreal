package desktop

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gioui.org/io/system"

	"github.com/northlighthq/boreal/internal/core/update"
	"github.com/northlighthq/boreal/internal/core/version"
	"github.com/northlighthq/boreal/internal/desktop/base"
)

const (
	updateDelay = 20 * time.Second
	updateEvery = 24 * time.Hour
)

type updater struct {
	mu    sync.Mutex
	info  base.UpdateInfo
	rel   update.Release
	path  string
	kind  update.Kind
	busy  bool
	shown float32
}

func updateDir() string {
	return filepath.Join(os.TempDir(), "boreal-update")
}

func (a *App) startUpdates() {
	update.Cleanup(updateDir())

	if update.Dev(version.Version) {
		return
	}

	go func() {
		time.Sleep(updateDelay)

		for {
			if a.autoUpdate.Load() {
				a.checkUpdates(false)
			}

			time.Sleep(updateEvery)
		}
	}()
}

func (a *App) Update() base.UpdateInfo {
	u := &a.updates
	u.mu.Lock()
	defer u.mu.Unlock()

	return u.info
}

func (a *App) CheckForUpdates() {
	go a.checkUpdates(true)
}

func (a *App) checkUpdates(manual bool) {
	u := &a.updates
	u.mu.Lock()

	if u.busy || u.info.State >= base.UpdateDownloading && u.info.State != base.UpdateFailed {
		u.mu.Unlock()
		return
	}

	prev := u.info
	u.busy = true
	u.info.State = base.UpdateChecking
	u.mu.Unlock()
	a.Invalidate()

	rel, newer, err := update.Check(context.Background(), version.Version)

	u.mu.Lock()
	u.busy = false

	switch {
	case err != nil && manual:
		u.info = base.UpdateInfo{State: base.UpdateFailed, Err: "Couldn't reach GitHub."}
	case err != nil:
		u.info = prev
	case newer:
		u.rel = rel
		u.info = base.UpdateInfo{State: base.UpdateAvailable, Version: rel.Version}
	default:
		u.info = base.UpdateInfo{State: base.UpdateCurrent}
	}

	info := u.info
	u.mu.Unlock()
	a.Invalidate()

	switch {
	case info.State == base.UpdateAvailable && prev.Version != info.Version:
		a.toasts.Info("Boreal "+info.Version+" is available", "Download it from the sidebar or Settings › About.")
	case info.State == base.UpdateCurrent && manual:
		a.toasts.Success("Boreal is up to date", "You have version "+version.Version+".")
	}
}

func (a *App) DownloadUpdate() {
	u := &a.updates
	u.mu.Lock()

	if u.rel.Version == "" || (u.info.State != base.UpdateAvailable && u.info.State != base.UpdateFailed) {
		u.mu.Unlock()
		return
	}

	rel := u.rel
	u.info = base.UpdateInfo{State: base.UpdateDownloading, Version: rel.Version}
	u.shown = 0
	u.mu.Unlock()
	a.Invalidate()

	go func() {
		kind := update.Detect()
		path, err := rel.Download(context.Background(), kind, updateDir(), func(done, total int64) {
			if total <= 0 {
				return
			}

			p := float32(done) / float32(total)
			u.mu.Lock()
			u.info.Progress = p
			redraw := p-u.shown >= 0.01 || p >= 1

			if redraw {
				u.shown = p
			}

			u.mu.Unlock()

			if redraw {
				a.Invalidate()
			}
		})

		if err == nil {
			err = update.Prepare(path, kind)
		}

		u.mu.Lock()

		if err != nil {
			u.info = base.UpdateInfo{State: base.UpdateFailed, Version: rel.Version, Err: err.Error()}
		} else {
			u.path, u.kind = path, kind
			u.info = base.UpdateInfo{State: base.UpdateReady, Version: rel.Version, Progress: 1}
		}

		u.mu.Unlock()
		a.Invalidate()

		if err != nil {
			a.toasts.Error("Couldn't download the update", err.Error())
			return
		}

		a.toasts.Success("Update ready", "Boreal "+rel.Version+" installs when you restart or close Boreal.")
	}()
}

func (a *App) RestartToUpdate() {
	if a.Update().State == base.UpdateReady {
		a.win.Perform(system.ActionClose)
	}
}

func (a *App) applyUpdate() {
	u := &a.updates
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.info.State != base.UpdateReady {
		return
	}

	update.Launch(u.path, u.kind)
}
