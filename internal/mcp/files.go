package mcp

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/northlighthq/boreal/internal/core/remotefs"
)

const (
	toolListFiles   = "list_files"
	toolReadFile    = "read_file"
	toolWriteFile   = "write_file"
	defaultReadSize = 64 << 10
	fileTimeout     = time.Minute
	encodingText    = "utf-8"
	encodingBase64  = "base64"
)

var (
	errNoFiles     = errors.New("working with files on hosts isn't allowed. It can be allowed in " + settingsPath)
	errNoWrite     = errors.New("writing files on hosts isn't allowed. It can be allowed in " + settingsPath)
	errEncoding    = errors.New(`encoding must be "utf-8" or "base64"`)
	errBadBase64   = errors.New("the content isn't valid base64")
	errFileTimeout = errors.New("the server took too long, so Boreal stopped")
)

type ListFilesIn struct {
	Host string `json:"host" jsonschema:"id or label of a host from list_hosts"`
	Path string `json:"path,omitempty" jsonschema:"folder to list. Relative paths start in the home folder of the user Boreal signs in as. The home folder by default"`
}

type FileEntry struct {
	Name     string    `json:"name"`
	Type     string    `json:"type"`
	Size     int64     `json:"size"`
	Mode     string    `json:"mode"`
	Modified time.Time `json:"modified"`
}

type ListFilesOut struct {
	Host      string      `json:"host"`
	Path      string      `json:"path"`
	Entries   []FileEntry `json:"entries"`
	Truncated bool        `json:"truncated,omitempty"`
}

type ReadFileIn struct {
	Host     string `json:"host" jsonschema:"id or label of a host from list_hosts"`
	Path     string `json:"path" jsonschema:"file to read. Relative paths start in the home folder"`
	Offset   int64  `json:"offset,omitempty" jsonschema:"byte to start reading at, to read a large file in parts"`
	MaxBytes int    `json:"maxBytes,omitempty" jsonschema:"most bytes to return, up to 1048576. 65536 by default"`
}

type ReadFileOut struct {
	Host      string `json:"host"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Offset    int64  `json:"offset"`
	Encoding  string `json:"encoding"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated,omitempty"`
}

type WriteFileIn struct {
	Host      string `json:"host" jsonschema:"id or label of a host from list_hosts"`
	Path      string `json:"path" jsonschema:"file to write. Relative paths start in the home folder. Its folder must exist"`
	Content   string `json:"content" jsonschema:"the whole new content of the file"`
	Encoding  string `json:"encoding,omitempty" jsonschema:"utf-8 (the default) or base64 for binary content"`
	Overwrite bool   `json:"overwrite,omitempty" jsonschema:"replace the file if it already exists"`
}

type WriteFileOut struct {
	Host    string `json:"host"`
	Path    string `json:"path"`
	Bytes   int    `json:"bytes"`
	Created bool   `json:"created"`
}

func (s *Server) addFiles(srv *mcp.Server) {
	closed, destructive := false, true
	reads := &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closed}

	mcp.AddTool(srv, &mcp.Tool{Name: toolListFiles, Description: fmt.Sprintf("List a folder on a host shared over MCP, folders first. Shows at most %d entries.", remotefs.MaxEntries), Annotations: reads}, s.listFiles)
	mcp.AddTool(srv, &mcp.Tool{Name: toolReadFile, Description: "Read a file on a host shared over MCP. Text comes back as is and binary content as base64. Read large files in parts with offset and maxBytes.", Annotations: reads}, s.readFile)
	mcp.AddTool(srv, &mcp.Tool{Name: toolWriteFile, Description: fmt.Sprintf("Create a file on a host shared over MCP, or replace one when overwrite is set. The file is written whole and swapped in at once, keeping its permissions. Up to %d KB. Files in ~/.ssh and authorized_keys can't be written: use grant_access and revoke_access.", remotefs.MaxWrite>>10), Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &closed}}, s.writeFile)
}

func (s *Server) recordFile(tool, host, p string, err error) {
	r := Record{Time: s.now(), Tool: tool, Host: host, Path: shortCommand(p), Outcome: OutcomeOK}

	if err != nil {
		r.Outcome, r.Error = OutcomeFail, err.Error()
	}

	s.log.Record(r)
}

func (s *Server) withFiles(ctx context.Context, t tier, host string, fn func(fs remotefs.FS, title string) error) error {
	v, err := s.load(t)

	if err != nil {
		return err
	}

	h, err := findHost(v, host)

	if err != nil {
		return err
	}

	fs, c, err := v.Browse(ctx, h)

	if err != nil {
		return err
	}

	defer c.Close()
	ctx, cancel := context.WithTimeout(ctx, fileTimeout)
	defer cancel()
	stop := context.AfterFunc(ctx, func() { c.Close() })
	err = fn(fs, h.Title())

	if !stop() && err != nil {
		return errFileTimeout
	}

	return err
}

func (s *Server) listFiles(ctx context.Context, _ *mcp.CallToolRequest, in ListFilesIn) (*mcp.CallToolResult, ListFilesOut, error) {
	var out ListFilesOut

	err := s.withFiles(ctx, tierFiles, in.Host, func(fs remotefs.FS, title string) error {
		dir, entries, truncated, err := fs.List(in.Path, 0)

		if err != nil {
			return err
		}

		out = ListFilesOut{Host: title, Path: dir, Entries: make([]FileEntry, 0, len(entries)), Truncated: truncated}

		for _, e := range entries {
			out.Entries = append(out.Entries, FileEntry{Name: e.Name, Type: e.Type, Size: e.Size, Mode: fmt.Sprintf("%04o", uint32(e.Mode)), Modified: e.Modified.UTC()})
		}

		return nil
	})

	s.recordFile(toolListFiles, in.Host, in.Path, err)
	return nil, out, err
}

func textPart(data []byte, truncated bool) ([]byte, bool) {
	if utf8.Valid(data) {
		return data, true
	}

	if !truncated {
		return nil, false
	}

	for cut := 1; cut < utf8.UTFMax && cut <= len(data); cut++ {
		if utf8.Valid(data[:len(data)-cut]) {
			return data[:len(data)-cut], true
		}
	}

	return nil, false
}

func (s *Server) readFile(ctx context.Context, _ *mcp.CallToolRequest, in ReadFileIn) (*mcp.CallToolResult, ReadFileOut, error) {
	var out ReadFileOut
	limit := in.MaxBytes

	if limit <= 0 {
		limit = defaultReadSize
	}

	err := s.withFiles(ctx, tierFiles, in.Host, func(fs remotefs.FS, title string) error {
		f, err := fs.Read(in.Path, in.Offset, limit)

		if err != nil {
			return err
		}

		out = ReadFileOut{Host: title, Path: f.Path, Size: f.Size, Offset: max(in.Offset, 0), Truncated: f.Truncated}

		if text, ok := textPart(f.Data, f.Truncated); ok {
			out.Encoding, out.Content = encodingText, string(text)
			return nil
		}

		out.Encoding, out.Content = encodingBase64, base64.StdEncoding.EncodeToString(f.Data)
		return nil
	})

	s.recordFile(toolReadFile, in.Host, in.Path, err)
	return nil, out, err
}

func decode(content, encoding string) ([]byte, error) {
	switch encoding {
	case "", encodingText:
		return []byte(content), nil
	case encodingBase64:
		b, err := base64.StdEncoding.DecodeString(content)

		if err != nil {
			return nil, errBadBase64
		}

		return b, nil
	}

	return nil, errEncoding
}

func (s *Server) writeFile(ctx context.Context, _ *mcp.CallToolRequest, in WriteFileIn) (*mcp.CallToolResult, WriteFileOut, error) {
	var out WriteFileOut
	data, err := decode(in.Content, in.Encoding)

	if err == nil && len(data) > remotefs.MaxWrite {
		err = remotefs.ErrTooBig
	}

	if err == nil {
		err = s.withFiles(ctx, tierWriteFiles, in.Host, func(fs remotefs.FS, title string) error {
			w, err := fs.Write(in.Path, data, in.Overwrite)

			if err != nil {
				return err
			}

			out = WriteFileOut{Host: title, Path: w.Path, Bytes: w.Bytes, Created: w.Created}
			return nil
		})
	}

	s.recordFile(toolWriteFile, in.Host, in.Path, err)
	return nil, out, err
}
