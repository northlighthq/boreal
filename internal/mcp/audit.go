package mcp

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/northlighthq/boreal/internal/core/config"
)

const (
	LogName     = "mcp.log"
	tailBytes   = 64 << 10
	maxLogSize  = 1 << 20
	oldLogName  = "mcp.log.1"
	logFileMode = 0o600
	OutcomeOK   = "ok"
	OutcomeFail = "error"
)

type Record struct {
	Time    time.Time `json:"time"`
	Tool    string    `json:"tool"`
	Host    string    `json:"host,omitempty"`
	Key     string    `json:"key,omitempty"`
	Command string    `json:"command,omitempty"`
	Outcome string    `json:"outcome"`
	Error   string    `json:"error,omitempty"`
}

type Recorder interface {
	Record(r Record)
}

type FileLog struct {
	mu   sync.Mutex
	path string
}

func NewFileLog() (*FileLog, error) {
	dir, err := config.Dir()

	if err != nil {
		return nil, err
	}

	return &FileLog{path: filepath.Join(dir, LogName)}, nil
}

func (l *FileLog) Record(r Record) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if info, err := os.Stat(l.path); err == nil && info.Size() > maxLogSize {
		os.Rename(l.path, filepath.Join(filepath.Dir(l.path), oldLogName))
	}

	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, logFileMode)

	if err != nil {
		return
	}

	defer f.Close()
	b, err := json.Marshal(r)

	if err != nil {
		return
	}

	f.Write(append(b, '\n'))
}

func Recent(n int) []Record {
	dir, err := config.Dir()

	if err != nil {
		return nil
	}

	f, err := os.Open(filepath.Join(dir, LogName))

	if err != nil {
		return nil
	}

	defer f.Close()

	if info, err := f.Stat(); err == nil && info.Size() > tailBytes {
		if _, err := f.Seek(info.Size()-tailBytes, io.SeekStart); err != nil {
			return nil
		}
	}

	var all []Record
	sc := bufio.NewScanner(f)

	for sc.Scan() {
		var r Record

		if json.Unmarshal(sc.Bytes(), &r) == nil {
			all = append(all, r)
		}
	}

	if len(all) > n {
		all = all[len(all)-n:]
	}

	return all
}
