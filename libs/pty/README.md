# pty

Start processes inside a pseudo terminal, with resize and clean shutdown. It uses ConPTY on Windows and `/dev/ptmx` on Linux and macOS, with no cgo. On other platforms `Start` returns `ErrUnsupported`.

It runs the local shells in [Boreal](https://github.com/northlighthq/boreal).

```sh
go get github.com/northlighthq/pty
```

```go
p, err := pty.Start(pty.Options{
	Command: []string{"bash", "-l"},
	Cols:    120,
	Rows:    30,
})

if err != nil {
	log.Fatal(err)
}

defer p.Close()
go io.Copy(os.Stdout, p)
p.Resize(100, 40)
code, _ := p.Wait()
```

With no `Command`, it starts the user's shell: `$SHELL` on Unix, PowerShell or `cmd.exe` on Windows.

Requires Windows 10 1809 or later on Windows.

## License

MIT
