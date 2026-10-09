# AGENTS.md

This file provides guidance to AI coding agents (Claude Code, Codex, etc.) when working with code in this repository.

## What this is

`termoscope` is a Go library plus a small CLI for testing terminal (TUI) programs. A test starts a binary under a
headless PTY, reads the emulated screen cell by cell, waits for screen states, and leaves PNG, static SVG and
animated SVG artifacts of every run under `test/results/`. The animated SVG doubles as README/website media.
Module path: `github.com/dimmkirr/termoscope`, Go 1.26, Unix-only (needs PTYs).

## Commands

```sh
task test                    # go test -race -count=1 ./...   (cgo needed for -race; CC=cc on nix)
task lint                    # gofmt -l check (excludes .devcell/), go vet, golangci-lint
task example                 # re-record docs/assets/countdown.svg + .png via the CLI
task tidy                    # go mod tidy

go test ./svg/ -run TestRender -v              # single package / test
go test ./test/e2e/... -count=1 -v                 # worked example; artifacts in test/results/<ts>-<Test>/
CC=cc go test -race ./...                          # on nix, -race needs the clang wrapper
go run ./cmd/termoscope record -o out.svg -cols 40 -rows 4 -- go run ./examples/countdown
```

Linters enabled in `.golangci.yml`: govet, staticcheck, errcheck, revive, gofmt.
`svg/fontembed_test.go` renders an SVG with headless Chromium to prove the embedded font is used; it skips when
no `chromium`/`google-chrome` is on PATH, so a green run without a browser has not exercised that path.

`.devcell/` is dev-container tooling unrelated to this module (contains stray Go files); never include it in
gofmt/vet sweeps and never edit it. `test/results/` and `.scratch/` are gitignored outputs.

## Architecture

Strict one-way dependency flow: `termoscope` (root) imports `raster`, `svg` and `gif`; `gif` imports
`raster` and `svg`; `raster` imports `internal/fonts` and `internal/twemoji`; `svg` imports only
`internal/fonts` and `ultraviolet` cell types. `raster`, `svg` and `gif` are pure (no files, no
`testing`); all file writing and `*testing.T` plumbing lives in the root package.

- **Root `termoscope`** (`terminal.go`, `artifacts.go`, `options.go`)
  - `Start` creates an `xpty` PTY and a `vt.SafeEmulator`, sets `TERM=xterm-256color COLORTERM=truecolor
    CLICOLOR_FORCE=1`, and runs two goroutines: one pumps PTY output into the emulator under `mu`, the other waits
    on the process and closes `exited`. Every screen read (`Screen`, `CellAt`, `Line`) takes `mu`; `CellAt`
    returns a copy because the child keeps writing.
  - `WaitFor`/`WaitUntil` poll every 20ms; the timeout error embeds the current screen text on purpose.
  - `artifacts.go` resolves the results root by walking up from cwd to `go.mod` (override with `SetResultsRoot`),
    and `Dir(t)` yields `test/results/<runID>-<testName>` where `runID` is one UTC timestamp fixed per test binary
    run. `Record` starts `svg.Record` in a goroutine and writes `recording.svg` from `t.Cleanup`, closing the
    terminal first so the recording ends where the test ends. The same frames also become `recording.gif` via
    `gif`. `RecordWith` takes `termoscope.Options` (`Interval`, `Hold`, `MinCols`, `MinRows`, `EmbedEmoji`);
    `options.go` maps it onto `svg.Options` and `gif.Options` so a test never imports the renderer packages and
    both recordings share one setting. Add new shared knobs there, not to the renderers' option structs alone.
- **`raster`**: screen to `*image.RGBA` at 2x using the embedded TTFs via `x/image/font/opentype`. Cell geometry
  constants (`FontSize=32`, `CellWidth=19`, `CellHeight=38`) encode svg's default 16px font at 2x with a 0.6em
  advance and a 1.2em line (19.2 and 38.4 rounded), so a GIF or PNG shown at half size lines up with the SVG
  to within 1%. Per cell: if the cell is 2 wide, contains U+FE0F, or its first rune is missing from JetBrains
  Mono's cmap, it is emoji. Emoji are drawn from `internal/twemoji` by whole grapheme cluster (color, default),
  scaled with CatmullRom into an em-sized square (`FontSize` px, what the SVG's emoji text occupies) centered in
  the `Width`-cell span and cached per cluster; with
  `Options.MonoEmoji`, or when Twemoji has no picture, the first rune is drawn with the Noto Emoji face in the
  fg color. The gate on JetBrains Mono's cmap keeps `#`, `*`, `©` as text even though Twemoji has pictures.
- **`internal/twemoji`**: `go:embed` of the Twemoji 17.0.3 72px PNG set (4009 files, about 4 MB, CC-BY 4.0,
  keep `LICENSE-GRAPHICS`). `Lookup(cluster)` maps a cluster to a file stem of hex code points joined by `-`,
  trying the exact sequence, then without U+FE0F, then the base code point; decoded images are cached.
- **`svg`**: `Snapshot` copies a `Screen` into a `Frame` (skips width-0 wide-glyph continuations, keeps
  content, fg color and `Width`); `Record` samples until `Done()` closes and drops identical consecutive frames;
  `Render` emits one `<g>` per frame toggled by a CSS keyframe animation (what GitHub renders inside `<img>`),
  `RenderStatic` emits one frame. Geometry derives from `fonts.Advance`/`fonts.CapHeight`: block glyphs `▄ ▀ █`
  become rounded squares, `─` a thin bar, text stays as `<text>`. `writeFrame` tracks the column separately from
  the slice index because wide cells occupy two columns but one slice entry; each wide glyph is its own centered
  `<text>` so its natural advance never shifts the grid. The font-family stack is JetBrains Mono, then (if
  `Options.EmbedEmoji`) Noto Emoji, then the platform color emoji fonts, then `monospace`. JetBrains Mono woff2 is
  embedded as a data URI unless `Options.NoEmbed`; Noto Emoji woff only with `EmbedEmoji` (about 750 KB).
- **`gif`**: animated GIF from `svg.Frame`s. A `frameScreen` adapter maps each frame back onto
  `raster.Screen` (column to cell index, continuation columns as width-0 cells) so `raster.Render` produces the
  pixels; `Scale: 2` (default) keeps the hi-DPI raster, `Scale: 1` halves it with CatmullRom. `padToMargin` then grows the
  `raster.Pad` then adds `Padding` (default `raster.Margin`, one cell height in 2x px, halved at Scale 1) on every
  side, so the GIF canvas equals the SVG canvas. Leading content-less frames (the empty screen sampled before
  the program printed) are dropped and timestamps rebased, so static GIF previews show content.
  `termoscope.SavePNG` and the CLI PNG path do the same crop and pad via
  `raster.Trim` (crop to content cells, honoring min size) followed by `raster.Pad`. The palette is exact while the recording
  has at most 256 distinct colors, else the 40 most frequent colors plus a 6x6x6 cube. Delays are centiseconds
  between frame timestamps, the last frame holds for `Hold`, `LoopCount` 0. Depends on `raster` and `svg`;
  `svg` must never import it.
- **`internal/fonts`**: `go:embed` of JetBrains Mono (TTF for raster, WOFF2 for SVG) and Noto Emoji Regular (static
  instance from Google Fonts; TTF for raster, WOFF for SVG) plus the em metrics both renderers share. Both fonts
  are OFL-licensed; keep `OFL.txt` and `OFL-NotoEmoji.txt` alongside. Go's `x/image` cannot render color fonts
  (CBDT/COLR/sbix), which is why the bundled emoji face is the monochrome Noto Emoji.
- **`cmd/termoscope`**: `record` subcommand wrapping `Start` + `svg.Record` + `raster.Render`. Exit code is the
  child's; the recording is written regardless.

The `Screen` interface (`Width`, `Height`, `CellAt`) is duplicated in `raster` and `svg` rather than shared so
each stays dependency-free; `*Terminal` and `*vt.Emulator` both satisfy it, which is how unit tests render without
a PTY.

## Conventions

- Tests that need a real program build an `examples/*` program into `t.TempDir()` (see `test/e2e`; `countdown`
  covers tiles and colors, `emoji` covers wide cells and the emoji font fallback). Pass `-pace` to keep runs
  short and `-buildvcs=false` when building from a tmp dir. Each example must fit 40x4 cells because
  `task example` and the CI workflows record every `examples/*/` at that size; `task example` also passes
  `-min-cols 20 -min-rows 4` so every README recording shares one canvas size (`Options.MinCols/MinRows`).
- When changing SVG or raster geometry, run `task example` and inspect `docs/assets/*.svg` in a browser;
  librsvg-based viewers ignore the embedded font and are not representative. A container without any emoji
  font shows tofu for emoji in non-embedded SVGs; that is the viewer, not a bug.
- README's Quick start is a verbatim copy of `test/e2e/example_countdown_test.go`; keep them in sync.
