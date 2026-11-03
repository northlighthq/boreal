package dragdrop

import (
	"encoding/binary"
	"io"
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	tymedIStream = 4
	dataDirGet   = 1
	gmemMoveable = 0x0002
	gmemZeroInit = 0x0040
	vkLButton    = 0x01
	keyDown      = 0x8000
	sFalse       = 1
	eNotImpl     = 0x80004001
	eInvalidArg  = 0x80070057
	eOutOfMemory = 0x8007000e
	dvEFormatEtc = 0x80040064
	dvELIndex    = 0x80040068
	oleENoAdvise = 0x80040003
	stgEDenied   = 0x80030005
	stgERead     = 0x8003001e
	stgTyStream  = 2

	fdAttributes  = 0x00000004
	fdWritesTime  = 0x00000020
	fdFileSize    = 0x00000040
	fdProgressUI  = 0x00004000
	fdUnicode     = 0x80000000
	attrDirectory = 0x10
	attrNormal    = 0x80
	maxPath       = 260
	descriptorLen = 592
	nameOffset    = 72
)

var (
	user32                   = windows.NewLazySystemDLL("user32.dll")
	kernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	procRegisterFormat       = user32.NewProc("RegisterClipboardFormatW")
	procGetAsyncKeyState     = user32.NewProc("GetAsyncKeyState")
	procGlobalAlloc          = kernel32.NewProc("GlobalAlloc")
	procGlobalLock           = kernel32.NewProc("GlobalLock")
	procGlobalUnlock         = kernel32.NewProc("GlobalUnlock")
	procSHCreateStdEnumFmtEt = shell32.NewProc("SHCreateStdEnumFmtEtc")

	iidStream           = windows.GUID{Data1: 0x0000000c, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidSequentialStream = windows.GUID{Data1: 0x0c733a30, Data2: 0x2a1c, Data3: 0x11ce, Data4: [8]byte{0xad, 0xe5, 0x00, 0xaa, 0x00, 0x44, 0x77, 0x3d}}

	formatsOnce              sync.Once
	cfDescriptor, cfContents uint16

	live   = map[uintptr]any{}
	liveMu sync.Mutex
)

type statStg struct {
	name     uintptr
	kind     uint32
	size     uint64
	mtime    windows.Filetime
	ctime    windows.Filetime
	atime    windows.Filetime
	mode     uint32
	locks    uint32
	clsid    windows.GUID
	state    uint32
	reserved uint32
}

type dataVtbl struct {
	queryInterface, addRef, release                    uintptr
	getData, getDataHere, queryGetData, getCanonical   uintptr
	setData, enumFormatEtc, dAdvise, dUnadvise, enumDA uintptr
}

type dataObject struct {
	vtbl  *dataVtbl
	refs  int32
	files []VirtualFile
}

type streamVtbl struct {
	queryInterface, addRef, release           uintptr
	read, write                               uintptr
	seek, setSize, copyTo, commit, revert     uintptr
	lockRegion, unlockRegion, stat, cloneFunc uintptr
}

type stream struct {
	vtbl *streamVtbl
	refs int32
	file VirtualFile
	r    io.ReadSeekCloser
}

func keep(p uintptr, v any) {
	liveMu.Lock()
	live[p] = v
	liveMu.Unlock()
}

func lookup(p uintptr) any {
	liveMu.Lock()
	defer liveMu.Unlock()

	return live[p]
}

func forget(p uintptr) {
	liveMu.Lock()
	delete(live, p)
	liveMu.Unlock()
}

func registerFormats() {
	formatsOnce.Do(func() {
		for name, dst := range map[string]*uint16{"FileGroupDescriptorW": &cfDescriptor, "FileContents": &cfContents} {
			p, _ := windows.UTF16PtrFromString(name)
			id, _, _ := procRegisterFormat.Call(uintptr(unsafe.Pointer(p)))
			*dst = uint16(id)
		}
	})
}

var dataTable = &dataVtbl{
	queryInterface: syscall.NewCallback(func(this uintptr, riid *windows.GUID, ppv *uintptr) uintptr {
		if *riid != iidUnknown && *riid != iidDataObject {
			*ppv = 0
			return eNoInterface
		}

		addRef(this)
		*ppv = this
		return sOK
	}),
	addRef:  syscall.NewCallback(addRef),
	release: syscall.NewCallback(release),
	getData: syscall.NewCallback(func(this uintptr, f *formatEtc, m *stgMedium) uintptr {
		d, ok := lookup(this).(*dataObject)

		if !ok || f == nil || m == nil {
			return eInvalidArg
		}

		return d.getData(f, m)
	}),
	getDataHere: syscall.NewCallback(func(this, f, m uintptr) uintptr { return eNotImpl }),
	queryGetData: syscall.NewCallback(func(this uintptr, f *formatEtc) uintptr {
		d, ok := lookup(this).(*dataObject)

		if !ok || f == nil {
			return eInvalidArg
		}

		return d.supports(f)
	}),
	getCanonical: syscall.NewCallback(func(this, in uintptr, out *formatEtc) uintptr {
		if out != nil {
			out.ptd = 0
		}

		return eNotImpl
	}),
	setData: syscall.NewCallback(func(this, f, m, release uintptr) uintptr { return eNotImpl }),
	enumFormatEtc: syscall.NewCallback(func(this, dir uintptr, out *uintptr) uintptr {
		if dir != dataDirGet {
			return eNotImpl
		}

		fmts := [2]formatEtc{
			{format: cfDescriptor, aspect: dvAspectContent, index: -1, tymed: tymedHGlobal},
			{format: cfContents, aspect: dvAspectContent, index: -1, tymed: tymedIStream},
		}
		hr, _, _ := procSHCreateStdEnumFmtEt.Call(uintptr(len(fmts)), uintptr(unsafe.Pointer(&fmts[0])), uintptr(unsafe.Pointer(out)))
		return hr
	}),
	dAdvise:   syscall.NewCallback(func(this, f, flags, sink, conn uintptr) uintptr { return oleENoAdvise }),
	dUnadvise: syscall.NewCallback(func(this, conn uintptr) uintptr { return oleENoAdvise }),
	enumDA:    syscall.NewCallback(func(this, out uintptr) uintptr { return oleENoAdvise }),
}

var streamTable = &streamVtbl{
	queryInterface: syscall.NewCallback(func(this uintptr, riid *windows.GUID, ppv *uintptr) uintptr {
		if *riid != iidUnknown && *riid != iidStream && *riid != iidSequentialStream {
			*ppv = 0
			return eNoInterface
		}

		addRef(this)
		*ppv = this
		return sOK
	}),
	addRef:  syscall.NewCallback(addRef),
	release: syscall.NewCallback(release),
	read: syscall.NewCallback(func(this uintptr, buf *byte, size uintptr, done *uint32) uintptr {
		s, ok := lookup(this).(*stream)

		if !ok || buf == nil {
			return eInvalidArg
		}

		return s.read(unsafe.Slice(buf, uint32(size)), done)
	}),
	write: syscall.NewCallback(func(this, buf, size, done uintptr) uintptr { return stgEDenied }),
	seek: syscall.NewCallback(func(this, move, origin uintptr, pos *uint64) uintptr {
		s, ok := lookup(this).(*stream)

		if !ok {
			return eInvalidArg
		}

		return s.seek(int64(move), int(uint32(origin)), pos)
	}),
	setSize: syscall.NewCallback(func(this, size uintptr) uintptr { return stgEDenied }),
	copyTo:  syscall.NewCallback(func(this, dst, size, read, written uintptr) uintptr { return eNotImpl }),
	commit:  syscall.NewCallback(func(this, flags uintptr) uintptr { return sOK }),
	revert:  syscall.NewCallback(func(this uintptr) uintptr { return sOK }),
	lockRegion: syscall.NewCallback(func(this, off, size, kind uintptr) uintptr {
		return eNotImpl
	}),
	unlockRegion: syscall.NewCallback(func(this, off, size, kind uintptr) uintptr {
		return eNotImpl
	}),
	stat: syscall.NewCallback(func(this uintptr, st *statStg, flags uintptr) uintptr {
		s, ok := lookup(this).(*stream)

		if !ok || st == nil {
			return eInvalidArg
		}

		*st = statStg{kind: stgTyStream, size: uint64(s.file.Size), mtime: filetime(s.file)}
		return sOK
	}),
	cloneFunc: syscall.NewCallback(func(this, out uintptr) uintptr { return eNotImpl }),
}

func addRef(this uintptr) uintptr {
	switch o := lookup(this).(type) {
	case *dataObject:
		o.refs++
		return uintptr(o.refs)
	case *stream:
		o.refs++
		return uintptr(o.refs)
	}

	return 0
}

func release(this uintptr) uintptr {
	switch o := lookup(this).(type) {
	case *dataObject:
		o.refs--

		if o.refs <= 0 {
			forget(this)
		}

		return uintptr(max(o.refs, 0))
	case *stream:
		o.refs--

		if o.refs <= 0 {
			forget(this)

			if o.r != nil {
				go o.r.Close()
			}
		}

		return uintptr(max(o.refs, 0))
	}

	return 0
}

func (d *dataObject) supports(f *formatEtc) uintptr {
	switch {
	case f.format == cfDescriptor && f.tymed&tymedHGlobal != 0:
		return sOK
	case f.format == cfContents && f.tymed&tymedIStream != 0:
		return sOK
	}

	return dvEFormatEtc
}

func (d *dataObject) getData(f *formatEtc, m *stgMedium) uintptr {
	if hr := d.supports(f); hr != sOK {
		return hr
	}

	if f.format == cfDescriptor {
		h, ok := descriptorGlobal(d.files)

		if !ok {
			return eOutOfMemory
		}

		*m = stgMedium{tymed: tymedHGlobal, hGlobal: h}
		return sOK
	}

	i := int(f.index)

	if i < 0 || i >= len(d.files) || d.files[i].Dir {
		return dvELIndex
	}

	s := &stream{vtbl: streamTable, refs: 1, file: d.files[i]}
	p := uintptr(unsafe.Pointer(s))
	keep(p, s)
	*m = stgMedium{tymed: tymedIStream, hGlobal: p}
	return sOK
}

func (s *stream) open() bool {
	if s.r != nil {
		return true
	}

	r, err := s.file.Open()

	if err != nil {
		return false
	}

	s.r = r
	return true
}

func (s *stream) read(buf []byte, done *uint32) uintptr {
	if !s.open() {
		return stgERead
	}

	n, err := io.ReadFull(s.r, buf)

	if done != nil {
		*done = uint32(n)
	}

	switch err {
	case nil:
		return sOK
	case io.EOF, io.ErrUnexpectedEOF:
		return sFalse
	}

	return stgERead
}

func (s *stream) seek(move int64, origin int, pos *uint64) uintptr {
	if !s.open() {
		return stgERead
	}

	n, err := s.r.Seek(move, origin)

	if err != nil {
		return eInvalidArg
	}

	if pos != nil {
		*pos = uint64(n)
	}

	return sOK
}

func filetime(f VirtualFile) windows.Filetime {
	if f.ModTime.IsZero() {
		return windows.Filetime{}
	}

	return windows.NsecToFiletime(f.ModTime.UnixNano())
}

func descriptors(files []VirtualFile) []byte {
	buf := make([]byte, 4+len(files)*descriptorLen)
	binary.LittleEndian.PutUint32(buf, uint32(len(files)))

	for i, f := range files {
		d := buf[4+i*descriptorLen : 4+(i+1)*descriptorLen]
		attrs := uint32(attrNormal)

		if f.Dir {
			attrs = attrDirectory
		}

		ft := filetime(f)
		binary.LittleEndian.PutUint32(d[0:], fdAttributes|fdWritesTime|fdFileSize|fdProgressUI|fdUnicode)
		binary.LittleEndian.PutUint32(d[36:], attrs)
		binary.LittleEndian.PutUint32(d[56:], ft.LowDateTime)
		binary.LittleEndian.PutUint32(d[60:], ft.HighDateTime)
		binary.LittleEndian.PutUint32(d[64:], uint32(uint64(f.Size)>>32))
		binary.LittleEndian.PutUint32(d[68:], uint32(f.Size))

		name := utf16.Encode([]rune(f.Name))

		if len(name) >= maxPath {
			name = name[:maxPath-1]
		}

		for j, c := range name {
			binary.LittleEndian.PutUint16(d[nameOffset+2*j:], c)
		}
	}

	return buf
}

func descriptorGlobal(files []VirtualFile) (uintptr, bool) {
	raw := descriptors(files)
	h, _, _ := procGlobalAlloc.Call(gmemMoveable|gmemZeroInit, uintptr(len(raw)))

	if h == 0 {
		return 0, false
	}

	p, _, _ := procGlobalLock.Call(h)

	if p == 0 {
		return 0, false
	}

	copy(unsafe.Slice((*byte)(unsafe.Add(nil, p)), len(raw)), raw)
	procGlobalUnlock.Call(h)
	return h, true
}

func DragVirtual(files []VirtualFile) error {
	if len(files) == 0 {
		return nil
	}

	if state, _, _ := procGetAsyncKeyState.Call(vkLButton); state&keyDown == 0 {
		return ErrReleased
	}

	procOleInitialize.Call(0)
	registerFormats()

	d := &dataObject{vtbl: dataTable, refs: 1, files: files}
	p := uintptr(unsafe.Pointer(d))
	keep(p, d)
	defer release(p)

	var effect uint32
	hr, _, _ := procDoDragDrop.Call(p, uintptr(unsafe.Pointer(source)), dropEffectCopy, uintptr(unsafe.Pointer(&effect)))

	if hr != dragDropSDrop && hr != dragDropSCancel {
		return windows.Errno(hr)
	}

	return nil
}
