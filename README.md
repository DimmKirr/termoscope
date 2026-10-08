# termoscope

Proof of what a terminal program showed. termoscope runs a binary under a
headless pseudo-terminal inside `go test`, lets you wait for and assert on
screen cells, and leaves a PNG, a static SVG, an animated SVG and an animated
GIF recording of every run for visual QA by humans or an LLM and for README
and website use.

<p>
  <img src="docs/assets/countdown.svg" alt="countdown">
  <img src="docs/assets/emoji.svg" alt="emoji">
  <img src="docs/assets/emoji.gif" alt="emoji as GIF" width="230" height="115">
</p>

- Real PTY via `charmbracelet/x/xpty`, screen via the pure Go `x/vt`
  emulator. No Chromium, no ffmpeg, no cgo at runtime.
- Cell-level reads: text, foreground and background colors, width.
- Waits that block on screen state instead of sleeping.
- Artifacts under `test/results/<dateTimeISO>-<testName>/`.
- JetBrains Mono, Twemoji and Noto Emoji are embedded, so PNG and GIF
  renders look the same on every machine, color emoji included. About
  5 MB of assets in the binary.
- A `termoscope record` CLI for recording README animations.

## Install

```sh
go get github.com/dimmkirr/termoscope
go install github.com/dimmkirr/termoscope/cmd/termoscope@latest
```

Requires Go 1.26 and a Unix-like OS that can open PTYs.

## Quick start

This is `test/e2e/example_countdown_test.go`, which drives
`examples/countdown`:

```go
func TestExample_Countdown(t *testing.T) {
	bin := buildCountdown(t) // go build -o <tmp> ../../examples/countdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tm, err := termoscope.Start(ctx, 40, 4, bin, "-pace=120ms")
	if err != nil {
		t.Fatal(err)
	}
	termoscope.Record(t, tm) // recording.svg written on cleanup

	if _, err := tm.WaitFor(ctx, "Countdown"); err != nil {
		t.Fatal(err)
	}
	termoscope.SavePNG(t, tm, "start")

	if _, err := tm.WaitFor(ctx, "Liftoff"); err != nil {
		t.Fatal(err)
	}
	if err := tm.Wait(); err != nil {
		t.Fatalf("countdown exited with %v", err)
	}
	termoscope.SavePNG(t, tm, "liftoff")
	termoscope.SaveSVG(t, tm, "liftoff")

	for _, x := range []int{11, 13, 15} {
		c := tm.CellAt(x, 0)
		if c == nil || c.Content != "▄" || !isGreen(c.Style.Fg) {
			t.Errorf("cell (%d,0) should be a green tile, got %+v", x, c)
		}
	}
}
```

Run it and look at the evidence:

```sh
go test ./test/e2e/... -count=1 -v
ls test/results/*/
# liftoff.png  liftoff.svg  recording.gif  recording.svg  start.png
```

## API

| Package | Purpose |
|---|---|
| `termoscope` | `Start`, `Terminal` (`Screen`, `Line`, `CellAt`, `Send`, `SendLine`, `WaitFor`, `WaitUntil`, `Done`, `Wait`, `Close`), `StripANSI`, test helpers `Dir`, `SavePNG`, `SaveSVG`, `Record`, `RecordWith`, `SetResultsRoot` |
| `raster` | Pure screen to `*image.RGBA` at 2x; `Render`, `RenderWith`, `Options{MonoEmoji}` |
| `svganim` | `Snapshot`, `Record`, `Render`, `RenderStatic`, `Options` (incl. `EmbedEmoji`), `Bounds` |
| `gifanim` | `Render` recorded frames as an animated GIF, `Options` (`Hold`, `Scale`, `Padding`, `MinCols`, `MinRows`, `MonoEmoji`) |

`Terminal` is safe to read from any goroutine while the program writes.
`CellAt` returns a copy. `WaitUntil` errors include the current screen, so a
failing wait shows what the user would have seen.

Artifacts land under the module root of the test being run, found by walking
up from the working directory to `go.mod`. `SetResultsRoot` overrides that.
Add `test/results/` to `.gitignore`.

## Recorder CLI

```sh
termoscope record -o demo.svg -png last.png -gif demo.gif -cols 80 -rows 24 -- ./myapp --flag
```

Flags: `-sample` (40ms), `-hold` (2s before the loop restarts), `-timeout`
(2m), `-font-size` (16), `-embed-emoji` (off), `-min-cols`/`-min-rows` (0, fit
content; the canvas and background grow to at least this many cells). The
exit code is the child's,
and the recording is written either way.

## SVG rendering

Geometry derives from the font: a cell is one glyph advance wide and two
tall. Block glyphs (`▄ ▀ █`) become rounded cap-height squares, the rule
`─` becomes a thin bar, and text keeps its native spacing. Frames switch
with a CSS keyframe animation, which GitHub renders inside `<img>`. The
font is embedded as a woff2 data URI; browsers honor it, librsvg does not.

## GIF rendering

`Record` also writes `recording.gif`, and `termoscope record -gif out.gif`
does the same from the CLI. Output is hi-DPI (2x) by default; `-gif-scale 1`
or `gifanim.Options{Scale: 1}` halves it for smaller files. Like the SVG, the
canvas is cropped to the content cells and padded by one cell height on every
side, and so is every PNG from `SavePNG` or `-png`
(`gifanim.Options.Padding`, `-padding`, in 2x pixels; -1 disables). The blank
screen sampled before the program printed is dropped so static previews of
the GIF show content. Frames are
rasterized with the bundled fonts and encoded with the standard library,
so the result is pixel-exact on every viewer, no browser or system font
involved. The trade-offs against SVG: fixed resolution and a palette of at
most 256 colors (exact when the screen uses few colors, quantized to a
6x6x6 cube plus the 40 most common colors otherwise).

## Emoji

The emulator gives emoji two columns, and all renderers keep the grid:
text after an emoji lands on its true column.

- **PNG and GIF** draw emoji in color from the bundled Twemoji pictures,
  looked up by the cell's whole grapheme cluster, so skin tones, flags and
  ZWJ sequences (👍🏽 🇨🇿 👨‍👩‍👧) render correctly with no text shaping
  and no system font. A cell counts as emoji when it is two columns wide,
  contains U+FE0F, or JetBrains Mono lacks its first code point, so `#`,
  `*` and `©` stay text. `raster.Options{MonoEmoji: true}`,
  `gifanim.Options{MonoEmoji: true}` or `-mono-emoji` switch to the
  monochrome Noto Emoji face tinted with the cell's foreground; it is also
  the fallback for anything Twemoji has no picture for.
- **SVG** emits each emoji as its own centered `<text>` and lists the
  platform color emoji fonts after JetBrains Mono: Twemoji Mozilla
  (Firefox), Apple Color Emoji (macOS, iOS), Segoe UI Emoji and Segoe UI
  Symbol (Windows), Noto Color Emoji (Android, ChromeOS, Linux), EmojiOne
  Color and Android Emoji (legacy). Browsers only consult them for glyphs
  JetBrains Mono lacks, so a README on GitHub shows the viewer's native
  color emoji. For a render that is identical everywhere, set
  `svganim.Options{EmbedEmoji: true}` or pass `-embed-emoji`: the
  monochrome Noto Emoji is embedded as a woff data URI, adding about
  750 KB.

## Testing

```sh
CC=cc go test -race ./...   # nix: cgo needs the clang wrapper
golangci-lint run ./...
```

`svganim` has two browser tests that render SVGs with headless Chromium to
prove the embedded fonts (JetBrains Mono, and Noto Emoji with `EmbedEmoji`)
are what the browser draws. They skip when no Chromium or Chrome is on
`PATH`.

## Consumers

- [go-streak-chart](https://github.com/dimmkirr/go-streak-chart), where this
  code was extracted from.

## License

MIT. JetBrains Mono and Noto Emoji are bundled under the SIL Open Font
License 1.1, see `internal/fonts/OFL.txt` and
`internal/fonts/OFL-NotoEmoji.txt`. Emoji pictures are
[Twemoji](https://github.com/jdecked/twemoji) 17.0.3 graphics, licensed
under [CC-BY 4.0](https://creativecommons.org/licenses/by/4.0/), see
`internal/twemoji/LICENSE-GRAPHICS`.
