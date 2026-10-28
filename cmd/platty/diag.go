package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"strconv"
	"time"

	platty "github.com/northlighthq/boreal/internal/app"
	"github.com/northlighthq/boreal/internal/config"
)

const (
	diagInterval = 3 * time.Second
	cpuWindow    = 15 * time.Second
)

func init() {
	if dir, err := config.Dir(); err == nil {
		if f, err := os.OpenFile(filepath.Join(dir, "crash.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
			fmt.Fprintf(f, "--- start %s (pid %d)\n", time.Now().Format(time.RFC3339), os.Getpid())
			debug.SetCrashOutput(f, debug.CrashOptions{})
		}
	}

	dir := os.Getenv("PLATTY_DIAG")

	if dir == "" {
		return
	}

	go func() {
		for {
			tmp := filepath.Join(dir, "cpu.tmp")
			f, err := os.Create(tmp)

			if err != nil {
				return
			}

			pprof.StartCPUProfile(f)
			time.Sleep(cpuWindow)
			pprof.StopCPUProfile()
			f.Close()
			os.Rename(tmp, filepath.Join(dir, "cpu.pprof"))
		}
	}()

	go func() {
		for {
			time.Sleep(diagInterval)
			n := platty.Frames.Swap(0)
			os.WriteFile(filepath.Join(dir, "fps.txt"), []byte(strconv.FormatFloat(float64(n)/diagInterval.Seconds(), 'f', 1, 64)), 0o644)

			if f, err := os.Create(filepath.Join(dir, "goroutines.txt")); err == nil {
				pprof.Lookup("goroutine").WriteTo(f, 2)
				f.Close()
			}

			writeMemDiag(dir)
		}
	}()
}

func writeMemDiag(dir string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	mb := func(b uint64) string { return strconv.FormatFloat(float64(b)/(1<<20), 'f', 1, 64) + " MB" }
	report := "heap in use: " + mb(m.HeapInuse) + "\n" +
		"heap allocated: " + mb(m.HeapAlloc) + "\n" +
		"stacks: " + mb(m.StackInuse) + "\n" +
		"total requested from OS: " + mb(m.Sys) + "\n" +
		"returned to OS: " + mb(m.HeapReleased) + "\n" +
		"GC: " + strconv.Itoa(int(m.NumGC)) + "\n"
	os.WriteFile(filepath.Join(dir, "mem.txt"), []byte(report), 0o644)

	if f, err := os.Create(filepath.Join(dir, "heap.pprof")); err == nil {
		pprof.Lookup("heap").WriteTo(f, 0)
		f.Close()
	}
}
