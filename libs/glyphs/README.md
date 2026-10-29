# glyphs

A cached glyph rasterizer for terminal grids: regular, bold and italic faces, system font fallback, COLR color emoji and ligature shaping.

It draws the text in [Boreal](https://github.com/northlighthq/boreal).

```sh
go get github.com/northlighthq/glyphs
```

- `NewSet` loads the four styles of a monospace font from an `fs.FS` and adds the system fallbacks.
- `NewCache` rasterizes glyphs at a pixel size and keeps them, so a full screen redraw only blends cached bitmaps.
- `Cache.Get` returns the bitmap for a rune in a style, spanning one or two cells.

## License

MIT
