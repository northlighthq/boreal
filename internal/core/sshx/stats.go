package sshx

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
)

const (
	maxStatsOutput = 256 * 1024
	maxStatsText   = 128
)

const statsCommand = `cat /proc/stat /proc/meminfo /proc/uptime /proc/loadavg /proc/net/dev 2>/dev/null; echo '@@df'; df -kP / 2>/dev/null | tail -1; echo '@@os'; uname -sr; hostname; grep -m1 PRETTY_NAME /etc/os-release 2>/dev/null`

type Stats struct {
	CPU                  float64
	Cores                int
	MemUsed              uint64
	MemTotal             uint64
	DiskUsed             uint64
	DiskTotal            uint64
	Load1, Load5, Load15 float64
	SwapUsed, SwapTotal  uint64
	DiskAvail            uint64
	RxTotal, TxTotal     uint64
	OS, Kernel, Hostname string
	Uptime               time.Duration
	RxRate               float64
	TxRate               float64
}

var ErrNoStats = errors.New("stats: the server has no /proc")

type raw struct {
	at             time.Time
	cpuTotal, idle uint64
	rx, tx         uint64
	stats          Stats
}

type Sampler struct {
	prev *raw
}

func (s *Sampler) Sample(c *Client) (Stats, error) {
	sess, err := c.NewSession()

	if err != nil {
		return Stats{}, err
	}

	defer sess.Close()
	stdout, err := sess.StdoutPipe()

	if err != nil {
		return Stats{}, err
	}

	if err := sess.Start(statsCommand); err != nil {
		return Stats{}, err
	}

	out, err := io.ReadAll(io.LimitReader(stdout, maxStatsOutput))

	if err != nil && len(out) == 0 {
		return Stats{}, err
	}

	r, err := parseStats(out)

	if err != nil {
		return Stats{}, err
	}

	r.at = time.Now()

	st := r.stats
	st.CPU, st.RxRate, st.TxRate = -1, -1, -1

	if p := s.prev; p != nil {
		if dt := r.cpuTotal - p.cpuTotal; dt > 0 && r.cpuTotal >= p.cpuTotal {
			st.CPU = 1 - float64(r.idle-p.idle)/float64(dt)
		}

		if secs := r.at.Sub(p.at).Seconds(); secs > 0 && r.rx >= p.rx && r.tx >= p.tx {
			st.RxRate = float64(r.rx-p.rx) / secs
			st.TxRate = float64(r.tx-p.tx) / secs
		}
	}

	s.prev = r
	return st, nil
}

func parseStats(out []byte) (*raw, error) {
	r := &raw{}
	var memAvail, swapFree uint64
	sawCPU := false
	section := ""
	osLine := 0

	sc := bufio.NewScanner(bytes.NewReader(out))

	for sc.Scan() {
		line := sc.Text()
		f := strings.Fields(line)

		if len(f) == 0 {
			continue
		}

		switch {
		case f[0] == "@@df" || f[0] == "@@os":
			section = f[0]
		case section == "@@df":
			if len(f) >= 4 {
				r.stats.DiskTotal = atou(f[1]) * 1024
				r.stats.DiskUsed = atou(f[2]) * 1024
				r.stats.DiskAvail = atou(f[3]) * 1024
			}
		case section == "@@os":
			switch {
			case strings.HasPrefix(line, "PRETTY_NAME="):
				r.stats.OS = clip(strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`))
			case osLine == 0:
				r.stats.Kernel = clip(line)
			case osLine == 1:
				r.stats.Hostname = clip(line)
			}

			osLine++
		case f[0] == "cpu":
			sawCPU = true

			for i, v := range f[1:] {
				n := atou(v)
				r.cpuTotal += n

				if i == 3 || i == 4 {
					r.idle += n
				}
			}
		case strings.HasPrefix(f[0], "cpu"):
			r.stats.Cores++
		case len(f) >= 2 && f[0] == "MemTotal:":
			r.stats.MemTotal = atou(f[1]) * 1024
		case len(f) >= 2 && f[0] == "MemAvailable:":
			memAvail = atou(f[1]) * 1024
		case len(f) >= 2 && f[0] == "SwapTotal:":
			r.stats.SwapTotal = atou(f[1]) * 1024
		case len(f) >= 2 && f[0] == "SwapFree:":
			swapFree = atou(f[1]) * 1024
		case len(f) == 2 && isFloat(f[0]) && isFloat(f[1]):
			secs, _ := strconv.ParseFloat(f[0], 64)
			r.stats.Uptime = time.Duration(secs) * time.Second
		case len(f) == 5 && strings.Contains(f[3], "/"):
			r.stats.Load1, _ = strconv.ParseFloat(f[0], 64)
			r.stats.Load5, _ = strconv.ParseFloat(f[1], 64)
			r.stats.Load15, _ = strconv.ParseFloat(f[2], 64)
		case strings.Contains(f[0], ":") && !strings.HasPrefix(f[0], "lo:"):
			name, rest, _ := strings.Cut(line, ":")

			if strings.TrimSpace(name) == "" || strings.HasSuffix(f[0], "Inter-|") {
				continue
			}

			nf := strings.Fields(rest)

			if len(nf) >= 9 {
				r.rx += atou(nf[0])
				r.tx += atou(nf[8])
			}
		}
	}

	if !sawCPU {
		return nil, ErrNoStats
	}

	if r.stats.MemTotal > memAvail {
		r.stats.MemUsed = r.stats.MemTotal - memAvail
	}

	if r.stats.SwapTotal > swapFree {
		r.stats.SwapUsed = r.stats.SwapTotal - swapFree
	}

	r.stats.RxTotal, r.stats.TxTotal = r.rx, r.tx
	return r, nil
}

func atou(s string) uint64 {
	n, _ := strconv.ParseUint(s, 10, 64)
	return n
}

func isFloat(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil && strings.Contains(s, ".")
}

func clip(s string) string {
	if r := []rune(s); len(r) > maxStatsText {
		return string(r[:maxStatsText])
	}

	return s
}
