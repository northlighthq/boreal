package transfer

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/sftp"
)

type Kind int

const (
	Upload Kind = iota
	Download
)

type State int

const (
	Queued State = iota
	Running
	Done
	Failed
	Cancelled
)

const parallel = 2

var (
	ErrCancelled     = errors.New("cancelled")
	errOutsideFolder = errors.New("the server listed a file outside the folder")
)

type Job struct {
	ID     int
	Kind   Kind
	Name   string
	Local  string
	Remote string
	HostID string
	Resume bool
	Writer io.WriteCloser

	client *sftp.Client
	total  atomic.Int64
	done   atomic.Int64
	files  atomic.Int64
	state  atomic.Int32
	err    error
	cancel context.CancelFunc
	ctx    context.Context
	onDone func(*Job)
}

func (j *Job) Progress() (done, total int64) {
	return j.done.Load(), j.total.Load()
}

func (j *Job) Files() int64 {
	return j.files.Load()
}

func (j *Job) State() State {
	return State(j.state.Load())
}

func (j *Job) Err() error {
	return j.err
}

func (j *Job) Finished() bool {
	s := j.State()
	return s == Done || s == Failed || s == Cancelled
}

type Queue struct {
	mu      sync.Mutex
	jobs    []*Job
	next    int
	slots   chan struct{}
	changed func()
	store   string
	pending []Saved
}

func New(changed func()) *Queue {
	if changed == nil {
		changed = func() {}
	}

	return &Queue{slots: make(chan struct{}, parallel), changed: changed}
}

func (q *Queue) Jobs() []*Job {
	q.mu.Lock()
	defer q.mu.Unlock()

	return append([]*Job(nil), q.jobs...)
}

func (q *Queue) Active() int {
	n := 0

	for _, j := range q.Jobs() {
		if !j.Finished() {
			n++
		}
	}

	return n
}

func (q *Queue) Add(client *sftp.Client, j *Job, onDone func(*Job)) *Job {
	q.mu.Lock()
	q.next++
	j.ID = q.next
	j.client = client
	j.onDone = onDone
	j.ctx, j.cancel = context.WithCancel(context.Background())
	q.jobs = append(q.jobs, j)
	q.mu.Unlock()

	go q.run(j)
	q.save()
	q.changed()
	return j
}

func (q *Queue) Cancel(id int) {
	for _, j := range q.Jobs() {
		if j.ID == id && !j.Finished() {
			j.cancel()
		}
	}
}

func (q *Queue) ClearFinished() {
	q.mu.Lock()
	live := q.jobs[:0]
	var stale []Saved
	busy := map[Saved]struct{}{}

	for _, j := range q.jobs {
		if !j.Finished() {
			live = append(live, j)
			busy[j.saved()] = struct{}{}
			continue
		}

		if j.State() == Failed && resumable(j) {
			stale = append(stale, j.saved())
		}
	}

	for _, s := range stale {
		if _, ok := busy[s]; !ok {
			s.discard()
		}
	}

	q.jobs = live
	q.mu.Unlock()
	q.save()
	q.changed()
}

func (q *Queue) run(j *Job) {
	select {
	case q.slots <- struct{}{}:
	case <-j.ctx.Done():
		j.finish(Cancelled, ErrCancelled)
		q.save()
		q.changed()
		return
	}

	defer func() { <-q.slots }()

	j.state.Store(int32(Running))
	q.changed()

	stop := make(chan struct{})

	go func() {
		t := time.NewTicker(200 * time.Millisecond)
		defer t.Stop()

		for {
			select {
			case <-stop:
				return
			case <-t.C:
				q.changed()
			}
		}
	}()

	var err error

	if j.Kind == Upload {
		err = j.upload()
	} else {
		err = j.download()
	}

	close(stop)

	switch {
	case err == nil:
		j.finish(Done, nil)
	case j.ctx.Err() != nil:
		j.finish(Cancelled, ErrCancelled)
	default:
		j.finish(Failed, err)
	}

	q.save()
	q.changed()
}

func (j *Job) finish(s State, err error) {
	j.err = err
	j.state.Store(int32(s))

	if j.onDone != nil {
		j.onDone(j)
	}
}

type counter struct {
	ctx context.Context
	n   *atomic.Int64
	r   io.Reader
}

func (c counter) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}

	n, err := c.r.Read(p)
	c.n.Add(int64(n))
	return n, err
}

func (j *Job) copy(dst io.Writer, src io.Reader) error {
	_, err := io.Copy(dst, counter{ctx: j.ctx, n: &j.done, r: src})
	return err
}

func (j *Job) upload() error {
	info, err := os.Stat(j.Local)

	if err != nil {
		return err
	}

	if !info.IsDir() {
		j.total.Store(info.Size())
		return j.uploadFile(j.Local, j.Remote)
	}

	var size int64

	filepath.WalkDir(j.Local, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if fi, err := d.Info(); err == nil {
				size += fi.Size()
			}
		}

		return nil
	})

	j.total.Store(size)

	return filepath.WalkDir(j.Local, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if err := j.ctx.Err(); err != nil {
			return err
		}

		rel, _ := filepath.Rel(j.Local, p)
		dst := path.Join(j.Remote, filepath.ToSlash(rel))

		if d.IsDir() {
			return j.client.MkdirAll(dst)
		}

		return j.uploadFile(p, dst)
	})
}

func (j *Job) uploadFile(src, dst string) error {
	in, err := os.Open(src)

	if err != nil {
		return err
	}

	defer in.Close()

	offset, out, err := j.openRemote(in, dst)

	if err != nil {
		return err
	}

	if fi, err := in.Stat(); err == nil && offset == fi.Size() {
		out.Close()
		j.done.Add(offset)
		j.files.Add(1)
		return nil
	}

	if _, err := in.Seek(offset, io.SeekStart); err != nil {
		out.Close()
		return err
	}

	j.done.Add(offset)
	err = j.copy(out, in)

	if cerr := out.Close(); err == nil {
		err = cerr
	}

	if err == nil {
		j.files.Add(1)
	}

	return err
}

func (j *Job) openRemote(in *os.File, dst string) (int64, *sftp.File, error) {
	if j.Resume {
		if offset := remoteResume(j.client, in, dst); offset > 0 {
			out, err := j.client.OpenFile(dst, os.O_WRONLY)

			if err == nil {
				if _, err = out.Seek(offset, io.SeekStart); err == nil {
					return offset, out, nil
				}

				out.Close()
			}
		}
	}

	out, err := j.client.Create(dst)
	return 0, out, err
}

func (j *Job) download() error {
	info, err := j.client.Stat(j.Remote)

	if err != nil {
		return err
	}

	if !info.IsDir() {
		j.total.Store(info.Size())

		if j.Writer != nil {
			defer j.Writer.Close()
			return j.downloadTo(j.Remote, j.Writer)
		}

		return j.downloadFile(j.Remote, j.Local, info.Size())
	}

	var size int64
	walk := j.client.Walk(j.Remote)

	for walk.Step() {
		if walk.Err() == nil && !walk.Stat().IsDir() {
			size += walk.Stat().Size()
		}
	}

	j.total.Store(size)
	walk = j.client.Walk(j.Remote)

	for walk.Step() {
		if err := walk.Err(); err != nil {
			return err
		}

		if err := j.ctx.Err(); err != nil {
			return err
		}

		dst, ok := localPath(j.Local, j.Remote, walk.Path())

		if !ok {
			return errOutsideFolder
		}

		if walk.Stat().IsDir() {
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return err
			}

			continue
		}

		if j.complete(dst, walk.Stat().Size()) {
			continue
		}

		if err := j.downloadFile(walk.Path(), dst, walk.Stat().Size()); err != nil {
			return err
		}
	}

	return nil
}

func (j *Job) complete(dst string, size int64) bool {
	if !j.Resume {
		return false
	}

	fi, err := os.Stat(dst)

	if err != nil || fi.Size() != size {
		return false
	}

	j.done.Add(size)
	j.files.Add(1)
	return true
}

func (j *Job) downloadFile(src, dst string, size int64) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	part := dst + PartSuffix
	in, err := j.client.Open(src)

	if err != nil {
		return err
	}

	defer in.Close()

	offset := localResume(part, in, size)
	flags := os.O_CREATE | os.O_WRONLY | os.O_TRUNC

	if offset > 0 {
		flags = os.O_WRONLY | os.O_APPEND
	}

	out, err := os.OpenFile(part, flags, 0o644)

	if err != nil {
		return err
	}

	if _, err := in.Seek(offset, io.SeekStart); err != nil {
		out.Close()
		return err
	}

	j.done.Add(offset)
	err = j.copy(out, in)

	if cerr := out.Close(); err == nil {
		err = cerr
	}

	if err == nil {
		err = os.Rename(part, dst)
	}

	if err != nil {
		if j.ctx.Err() != nil {
			os.Remove(part)
		}

		return err
	}

	j.files.Add(1)
	return nil
}

func (j *Job) downloadTo(src string, w io.Writer) error {
	in, err := j.client.Open(src)

	if err != nil {
		return err
	}

	defer in.Close()

	if err := j.copy(w, in); err != nil {
		return err
	}

	j.files.Add(1)
	return nil
}
