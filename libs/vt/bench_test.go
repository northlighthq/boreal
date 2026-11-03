package vt

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

const (
	benchCols = 200
	benchRows = 50
)

func logStream(lines int) []byte {
	var b bytes.Buffer

	for i := range lines {
		fmt.Fprintf(&b, "2026-11-03T10:%02d:%02d.%03dZ INFO  request id=%08x path=/api/v1/items/%d status=200 duration=%dms\r\n", i/60%60, i%60, i%1000, i*2654435761, i, i%250)
	}

	return b.Bytes()
}

func colorStream(lines int) []byte {
	var b bytes.Buffer
	names := []string{"main.go", "go.mod", "internal", "README.md", "bin", "libs", "LICENSE"}

	for i := range lines {
		for j, n := range names {
			fmt.Fprintf(&b, "\x1b[38;5;%dm%s\x1b[0m  \x1b[1;38;2;%d;%d;%dm%d\x1b[0m ", (i+j)%256, n, i%256, j*30, 200, i+j)
		}

		b.WriteString("\r\n")
	}

	return b.Bytes()
}

func fullScreenStream(frames int) []byte {
	var b bytes.Buffer
	b.WriteString("\x1b[?1049h")
	row := strings.Repeat("x", benchCols-20)

	for f := range frames {
		b.WriteString("\x1b[H")

		for y := 1; y <= benchRows; y++ {
			fmt.Fprintf(&b, "\x1b[%d;1H\x1b[%dm%5d %5.1f%% \x1b[0m%s\x1b[K", y, 30+(y+f)%8, f*y, float64(y)/3, row)
		}
	}

	b.WriteString("\x1b[?1049l")
	return b.Bytes()
}

func unicodeStream(lines int) []byte {
	var b bytes.Buffer

	for i := range lines {
		fmt.Fprintf(&b, "%d 漢字かなカナ한국어 😀👍🏽 café naïve — ✓ ✗ → ← │ ├── └── %d\r\n", i, i*7)
	}

	return b.Bytes()
}

func benchWrite(b *testing.B, data []byte) {
	t := New(benchCols, benchRows)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		t.Write(data)
	}
}

func BenchmarkWriteLog(b *testing.B)        { benchWrite(b, logStream(2000)) }
func BenchmarkWriteColor(b *testing.B)      { benchWrite(b, colorStream(500)) }
func BenchmarkWriteFullScreen(b *testing.B) { benchWrite(b, fullScreenStream(20)) }
func BenchmarkWriteUnicode(b *testing.B)    { benchWrite(b, unicodeStream(1000)) }

func BenchmarkSnapshot(b *testing.B) {
	t := New(benchCols, benchRows)
	t.Write(colorStream(200))
	var f Frame
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		t.Snapshot(&f, 0)
	}
}

func BenchmarkSnapshotScrolled(b *testing.B) {
	t := New(benchCols, benchRows)
	t.Write(logStream(5000))
	var f Frame
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		t.Snapshot(&f, 2000)
	}
}

func BenchmarkResizeReflow(b *testing.B) {
	data := logStream(10_000)
	b.ReportAllocs()

	for b.Loop() {
		b.StopTimer()
		t := New(benchCols, benchRows)
		t.Write(data)
		b.StartTimer()
		t.Resize(120, 40)
		t.Resize(benchCols, benchRows)
	}
}
