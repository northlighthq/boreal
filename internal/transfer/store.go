package transfer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/northlighthq/boreal/internal/config"
)

type Saved struct {
	Kind   Kind   `json:"kind"`
	Name   string `json:"name"`
	Local  string `json:"local"`
	Remote string `json:"remote"`
	HostID string `json:"hostId"`
}

func (s Saved) Job() *Job {
	return &Job{Kind: s.Kind, Name: s.Name, Local: s.Local, Remote: s.Remote, HostID: s.HostID, Resume: true}
}

func (j *Job) saved() Saved {
	return Saved{Kind: j.Kind, Name: j.Name, Local: j.Local, Remote: j.Remote, HostID: j.HostID}
}

func (s Saved) discard() {
	if s.Kind == Download && s.Local != "" {
		os.Remove(s.Local + PartSuffix)
	}
}

func (q *Queue) Persist(path string) {
	q.mu.Lock()
	q.store = path

	if raw, err := os.ReadFile(path); err == nil {
		json.Unmarshal(raw, &q.pending)
	}

	q.mu.Unlock()
	q.changed()
}

func (q *Queue) Pending() []Saved {
	q.mu.Lock()
	defer q.mu.Unlock()

	return append([]Saved(nil), q.pending...)
}

func (q *Queue) TakePending(hostID string) []Saved {
	if hostID == "" {
		return nil
	}

	q.mu.Lock()
	var out []Saved
	kept := q.pending[:0]

	for _, s := range q.pending {
		if s.HostID == hostID {
			out = append(out, s)
			continue
		}

		kept = append(kept, s)
	}

	q.pending = kept
	q.mu.Unlock()

	if len(out) > 0 {
		q.save()
	}

	return out
}

func (q *Queue) Dismiss(s Saved) {
	q.mu.Lock()

	for i, p := range q.pending {
		if p == s {
			q.pending = append(q.pending[:i], q.pending[i+1:]...)
			s.discard()
			break
		}
	}

	q.mu.Unlock()
	q.save()
	q.changed()
}

func (q *Queue) save() {
	q.mu.Lock()
	path := q.store
	out := append([]Saved(nil), q.pending...)
	seen := map[Saved]struct{}{}

	for _, s := range out {
		seen[s] = struct{}{}
	}

	for _, j := range q.jobs {
		if j.State() == Done {
			seen[j.saved()] = struct{}{}
		}
	}

	for _, j := range q.jobs {
		if s := j.State(); (s == Done || s == Cancelled) || !resumable(j) {
			continue
		}

		s := j.saved()

		if _, ok := seen[s]; ok {
			continue
		}

		seen[s] = struct{}{}
		out = append(out, s)
	}

	q.mu.Unlock()

	if path == "" {
		return
	}

	if len(out) == 0 {
		os.Remove(path)
		return
	}

	if raw, err := json.Marshal(out); err == nil {
		config.WriteAtomic(path, raw)
	}
}

func resumable(j *Job) bool {
	if j.Writer != nil || j.Local == "" || j.HostID == "" {
		return false
	}

	tmp, err := filepath.Abs(os.TempDir())

	if err != nil {
		return true
	}

	return !strings.HasPrefix(strings.ToLower(filepath.Clean(j.Local)), strings.ToLower(tmp)+string(filepath.Separator))
}
