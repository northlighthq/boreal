package sshx

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	pageantMaxMessage = 8192
	pageantCopyDataID = 0x804e50ba
	wmCopyData        = 0x004a
)

var (
	pageantUser32   = windows.NewLazySystemDLL("user32.dll")
	procFindWindow  = pageantUser32.NewProc("FindWindowW")
	procSendMessage = pageantUser32.NewProc("SendMessageW")

	pageantSeq atomic.Uint32

	errNoPageant      = errors.New("pageant isn't running")
	errPageantTooBig  = errors.New("pageant: message too large")
	errPageantRefused = errors.New("pageant refused the request")
)

type copyData struct {
	data uintptr
	size uint32
	ptr  uintptr
}

type pageant struct {
	hwnd uintptr
	req  bytes.Buffer
	resp bytes.Reader
}

func pageantWindow() uintptr {
	name, _ := windows.UTF16PtrFromString("Pageant")
	hwnd, _, _ := procFindWindow.Call(uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(name)))
	return hwnd
}

func dialPageant() (io.ReadWriteCloser, error) {
	hwnd := pageantWindow()

	if hwnd == 0 {
		return nil, errNoPageant
	}

	return &pageant{hwnd: hwnd}, nil
}

func (p *pageant) Write(b []byte) (int, error) {
	p.req.Write(b)

	if p.req.Len() < 4 {
		return len(b), nil
	}

	need := int(binary.BigEndian.Uint32(p.req.Bytes())) + 4

	if p.req.Len() < need {
		return len(b), nil
	}

	resp, err := p.query(p.req.Next(need))

	if err != nil {
		return 0, err
	}

	p.resp.Reset(resp)
	return len(b), nil
}

func (p *pageant) Read(b []byte) (int, error) {
	return p.resp.Read(b)
}

func (p *pageant) Close() error {
	return nil
}

func (p *pageant) query(msg []byte) ([]byte, error) {
	if len(msg) > pageantMaxMessage {
		return nil, errPageantTooBig
	}

	name := fmt.Sprintf("PageantRequest%08x%04x", windows.GetCurrentThreadId(), pageantSeq.Add(1)&0xffff)
	namePtr, err := windows.UTF16PtrFromString(name)

	if err != nil {
		return nil, err
	}

	mapping, err := windows.CreateFileMapping(windows.InvalidHandle, nil, windows.PAGE_READWRITE, 0, pageantMaxMessage, namePtr)

	if err != nil {
		return nil, err
	}

	defer windows.CloseHandle(mapping)

	addr, err := windows.MapViewOfFile(mapping, windows.FILE_MAP_WRITE, 0, 0, 0)

	if err != nil {
		return nil, err
	}

	defer windows.UnmapViewOfFile(addr)

	view := unsafe.Slice((*byte)(unsafe.Add(nil, addr)), pageantMaxMessage)
	copy(view, msg)

	ansi := append([]byte(name), 0)
	cds := copyData{data: pageantCopyDataID, size: uint32(len(ansi)), ptr: uintptr(unsafe.Pointer(&ansi[0]))}
	ok, _, _ := procSendMessage.Call(p.hwnd, wmCopyData, 0, uintptr(unsafe.Pointer(&cds)))
	runtime.KeepAlive(ansi)

	if ok == 0 {
		return nil, errPageantRefused
	}

	n := int(binary.BigEndian.Uint32(view)) + 4

	if n > pageantMaxMessage {
		return nil, errPageantTooBig
	}

	return append([]byte(nil), view[:n]...), nil
}
