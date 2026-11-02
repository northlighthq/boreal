package sshx

import (
	"testing"
)

const sampleStats = `cpu  4705 356 584 3699176 23060 0 277 0 0 0
cpu0 1393 280 330 920231 5788 0 37 0 0 0
cpu1 1088 33 99 925806 5779 0 199 0 0 0
intr 1462898
MemTotal:        8040256 kB
MemFree:          912412 kB
MemAvailable:    6823316 kB
SwapTotal:       2097148 kB
SwapFree:        2097148 kB
9405.29 36741.75
0.10 0.05 0.01 1/245 8123
Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo:  123456     100    0    0    0     0          0         0   123456     100    0    0    0     0       0          0
  eth0: 98765432  65432    0    0    0     0          0         0 12345678   43210    0    0    0     0       0          0
@@df
/dev/sda1 102350568 10180316 92153868 10% /
@@os
Linux 6.8.0-45-generic
web-01
PRETTY_NAME="Ubuntu 24.04.1 LTS"
`

func TestParseStats(t *testing.T) {
	r, err := parseStats([]byte(sampleStats))

	if err != nil {
		t.Fatal(err)
	}

	if r.stats.Cores != 2 || r.stats.MemTotal != 8040256*1024 || r.stats.Hostname != "web-01" || r.stats.OS != "Ubuntu 24.04.1 LTS" {
		t.Fatalf("unexpected stats: %+v", r.stats)
	}

	if r.rx != 98765432 || r.tx != 12345678 {
		t.Fatalf("unexpected network totals: rx %d tx %d", r.rx, r.tx)
	}
}

func TestParseStatsWithoutProc(t *testing.T) {
	if _, err := parseStats([]byte("@@df\n@@os\nDarwin 23.0\n")); err != ErrNoStats {
		t.Fatalf("got %v, want ErrNoStats", err)
	}
}

func FuzzParseStats(f *testing.F) {
	f.Add([]byte(sampleStats))
	f.Add([]byte("cpu\n@@os\n\n\n"))
	f.Add([]byte("cpu 1 2 3\n:x\n x: 1 2 3 4 5 6 7 8 9\n1.0 2.0\n"))

	f.Fuzz(func(t *testing.T, out []byte) {
		r, err := parseStats(out)

		if err != nil {
			return
		}

		if r.stats.MemUsed > r.stats.MemTotal || r.stats.SwapUsed > r.stats.SwapTotal {
			t.Fatalf("used exceeds total: %+v", r.stats)
		}

		if len([]rune(r.stats.OS)) > maxStatsText || len([]rune(r.stats.Kernel)) > maxStatsText || len([]rune(r.stats.Hostname)) > maxStatsText {
			t.Fatal("text not clipped")
		}
	})
}
