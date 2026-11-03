package dragdrop

import (
	"bytes"
	"encoding/binary"
	"io"
	"syscall"
	"testing"
	"time"
	"unicode/utf16"
	"unsafe"
)

type nopSeeker struct{ *bytes.Reader }

func (nopSeeker) Close() error { return nil }

func TestDescriptors(t *testing.T) {
	files := []VirtualFile{
		{Name: "logs", Dir: true},
		{Name: `logs\app.log`, Size: 5 << 32, ModTime: time.Unix(1_700_000_000, 0)},
	}
	raw := descriptors(files)

	if len(raw) != 4+2*descriptorLen || binary.LittleEndian.Uint32(raw) != 2 {
		t.Fatalf("bad header, len %d", len(raw))
	}

	d := raw[4+descriptorLen:]

	if binary.LittleEndian.Uint32(d[64:]) != 5 || binary.LittleEndian.Uint32(d[68:]) != 0 {
		t.Fatal("size not split into high and low words")
	}

	name := make([]uint16, len(`logs\app.log`))

	for i := range name {
		name[i] = binary.LittleEndian.Uint16(d[nameOffset+2*i:])
	}

	if string(utf16.Decode(name)) != `logs\app.log` {
		t.Fatalf("name = %q", string(utf16.Decode(name)))
	}

	if binary.LittleEndian.Uint32(raw[4+36:]) != attrDirectory {
		t.Fatal("folder entry must have the directory attribute")
	}
}

func TestStreamThroughVtable(t *testing.T) {
	registerFormats()
	content := []byte("hello from the server")
	d := &dataObject{vtbl: dataTable, refs: 1, files: []VirtualFile{{
		Name: "a.txt",
		Size: int64(len(content)),
		Open: func() (io.ReadSeekCloser, error) { return nopSeeker{bytes.NewReader(content)}, nil },
	}}}
	p := uintptr(unsafe.Pointer(d))
	keep(p, d)
	defer release(p)

	f := formatEtc{format: cfContents, aspect: dvAspectContent, index: 0, tymed: tymedIStream}
	var m stgMedium
	hr, _, _ := syscall.SyscallN(dataTable.getData, p, uintptr(unsafe.Pointer(&f)), uintptr(unsafe.Pointer(&m)))

	if hr != sOK || m.tymed != tymedIStream || m.hGlobal == 0 {
		t.Fatalf("GetData = %#x, medium %+v", hr, m)
	}

	buf := make([]byte, 64)
	var n uint32
	hr, _, _ = syscall.SyscallN(streamTable.read, m.hGlobal, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&n)))

	if hr != sFalse || string(buf[:n]) != string(content) {
		t.Fatalf("Read = %#x, %q", hr, buf[:n])
	}

	var st statStg
	syscall.SyscallN(streamTable.stat, m.hGlobal, uintptr(unsafe.Pointer(&st)), 0)

	if st.size != uint64(len(content)) {
		t.Fatalf("Stat size = %d", st.size)
	}

	syscall.SyscallN(streamTable.release, m.hGlobal)

	if lookup(m.hGlobal) != nil {
		t.Fatal("stream not released")
	}

	bad := formatEtc{format: cfContents, index: 3, tymed: tymedIStream}

	if hr, _, _ := syscall.SyscallN(dataTable.getData, p, uintptr(unsafe.Pointer(&bad)), uintptr(unsafe.Pointer(&m))); hr != dvELIndex {
		t.Fatalf("bad index = %#x", hr)
	}
}
