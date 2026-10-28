# vt

A VT terminal emulator for Go with no GUI attached: an escape sequence parser, screen and scrollback state, reflow on resize, text search and input encoding.

It powers the terminal in [Platty](https://github.com/PlattySSH/platty).

```sh
go get github.com/northlighthq/vt
```

```go
term := vt.New(80, 24)
term.Write([]byte("\x1b[1mhello\x1b[0m world\r\n"))
term.Resize(120, 30)
```

- `vt` keeps the screen, scrollback, cursor, modes and OSC state (links, working folder, clipboard).
- `vt/parser` turns a byte stream into print, execute, CSI, ESC, OSC and DCS events.
- `vt/input` encodes keys, mouse events, focus and bracketed paste for the program on the other side.

## License

MIT
