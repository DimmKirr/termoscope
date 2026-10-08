# AGENTS.md

This file provides guidance to AI coding agents (Claude Code, Codex, etc.) when working with code in this repository.

## What this is

`termproof` is a Go library plus a small CLI for testing terminal (TUI) programs. A test starts a binary under a
headless PTY, reads the emulated screen cell by cell, waits for screen states, and leaves PNG, static SVG and
animated SVG artifacts of every run under `test/results/`. The animated SVG doubles as README/website media.
Module path: `github.com/dimmkirr/termproof`, Go 1.26, Unix-only (needs PTYs).

## Commands

```sh
task test                    # go test -race -count=1 ./...   (cgo needed for -race; CC=cc on nix)
task lint                    # gofmt -l check (excludes .devcell/), go vet, golangci-lint
task example                 # re-record docs/assets/countdown.svg + .png via the CLI
task tidy                    # go mod tidy

go test ./svganim/ -run TestRender -v              # single package / test
go test ./test/e2e/... -count=1 -v                 # worked example; artifacts in test/results/<ts>-<Test>/
CC=cc go test -race ./...                          # on nix, -race needs the clang wrapper
go run ./cmd/termproof record -o out.svg -cols 40 -rows 4 -- go run ./examples/countdown
```

Linters enabled in `.golangci.yml`: govet, staticcheck, errcheck, revive, gofmt.
`svganim/fontembed_test.go` renders an SVG with headless Chromium to prove the embedded font is used; it skips when
no `chromium`/`google-chrome` is on PATH, so a green run without a browser has not exercised that path.

`.devcell/` is dev-container tooling unrelated to this module (contains stray Go files); never include it in
gofmt/vet sweeps and never edit it. `test/results/` and `.scratch/` are gitignored outputs.

## Architecture

Four packages with a strict one-way dependency flow: `termproof` (root) imports `raster` and `svganim`, both of
which import only `internal/fonts` and `ultraviolet` cell types. `raster` and `svganim` are pure (no files, no
`testing`); all file writing and `*testing.T` plumbing lives in the root package.

- **Root `termproof`** (`terminal.go`, `artifacts.go`)
  - `Start` creates an `xpty` PTY and a `vt.SafeEmulator`, sets `TERM=xterm-256color COLORTERM=truecolor
    CLICOLOR_FORCE=1`, and runs two goroutines: one pumps PTY output into the emulator under `mu`, the other waits
    on the process and closes `exited`. Every screen read (`Screen`, `CellAt`, `Line`) takes `mu`; `CellAt`
    returns a copy because the child keeps writing.
  - `WaitFor`/`WaitUntil` poll every 20ms; the timeout error embeds the current screen text on purpose.
  - `artifacts.go` resolves the results root by walking up from cwd to `go.mod` (override with `SetResultsRoot`),
    and `Dir(t)` yields `test/results/<runID>-<testName>` where `runID` is one UTC timestamp fixed per test binary
    run. `Record` starts `svganim.Record` in a goroutine and writes `recording.svg` from `t.Cleanup`, closing the
    terminal first so the recording ends where the test ends.
- **`raster`**: screen to `*image.RGBA` at 2x using the embedded TTFs via `x/image/font/opentype`. Cell geometry
  constants (`FontSize=28`, `CellWidth=17`, `CellHeight=35`) encode a 14px font at 2x with a 0.6em advance and
  1.25em line. Per cell it checks the first rune against JetBrains Mono's cmap; if missing and present in Noto
  Emoji, the glyph is drawn with the emoji face centered in the cell's `Width`-cell span. No shaping: clusters
  draw as their first rune.
- **`svganim`**: `Snapshot` copies a `Screen` into a `Frame` (skips width-0 wide-glyph continuations, keeps
  content, fg color and `Width`); `Record` samples until `Done()` closes and drops identical consecutive frames;
  `Render` emits one `<g>` per frame toggled by a CSS keyframe animation (what GitHub renders inside `<img>`),
  `RenderStatic` emits one frame. Geometry derives from `fonts.Advance`/`fonts.CapHeight`: block glyphs `▄ ▀ █`
  become rounded squares, `─` a thin bar, text stays as `<text>`. `writeFrame` tracks the column separately from
  the slice index because wide cells occupy two columns but one slice entry; each wide glyph is its own centered
  `<text>` so its natural advance never shifts the grid. The font-family stack is JetBrains Mono, then (if
  `Options.EmbedEmoji`) Noto Emoji, then the platform color emoji fonts, then `monospace`. JetBrains Mono woff2 is
  embedded as a data URI unless `Options.NoEmbed`; Noto Emoji woff only with `EmbedEmoji` (about 750 KB).
- **`internal/fonts`**: `go:embed` of JetBrains Mono (TTF for raster, WOFF2 for SVG) and Noto Emoji Regular (static
  instance from Google Fonts; TTF for raster, WOFF for SVG) plus the em metrics both renderers share. Both fonts
  are OFL-licensed; keep `OFL.txt` and `OFL-NotoEmoji.txt` alongside. Go's `x/image` cannot render color fonts
  (CBDT/COLR/sbix), which is why the bundled emoji face is the monochrome Noto Emoji.
- **`cmd/termproof`**: `record` subcommand wrapping `Start` + `svganim.Record` + `raster.Render`. Exit code is the
  child's; the recording is written regardless.

The `Screen` interface (`Width`, `Height`, `CellAt`) is duplicated in `raster` and `svganim` rather than shared so
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
