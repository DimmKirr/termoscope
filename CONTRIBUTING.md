# Contributing

Bug reports and pull requests are welcome at
https://github.com/dimmkirr/termoscope. Small, focused changes with a test
usually land within a day. If something termoscope should do is missing,
open an issue first only when the design is unclear; otherwise a PR is fine.

## Setup

```sh
git clone https://github.com/dimmkirr/termoscope
cd termoscope
task test      # go test -race -count=1 ./...  (CC=cc on nix)
task lint      # gofmt, go vet, golangci-lint
```

Requires Go 1.26 and a Unix-like OS that can open PTYs. `task example`
re-records `docs/assets/` from `examples/*`; run it and look at the SVGs
in a browser whenever a change touches rendering geometry.

## Where things live

The architecture is described in [AGENTS.md](AGENTS.md). In short:

| Change | Where |
|---|---|
| Driving the program, reading the screen, waits | `terminal.go` |
| Artifact paths, `SavePNG`, `SaveSVG`, `Record` | `artifacts.go` |
| A knob shared by the SVG and GIF recordings | `options.go` (`Options`), then map it in `svg.Options` and `gif.Options` |
| Pixels (PNG, GIF frames), cell geometry, emoji pictures | `raster/` |
| SVG markup, CSS animation, embedded fonts | `svg/` |
| GIF timing, palette, canvas padding | `gif/` |
| The `termoscope record` CLI | `cmd/termoscope/` |
| Fonts and Twemoji assets | `internal/fonts/`, `internal/twemoji/` |

Rules that keep the packages composable:

- `raster`, `svg` and `gif` are pure: no files, no `testing`. File writing
  and `*testing.T` plumbing stay in the root package.
- `svg` never imports `gif` or `raster`. `gif` may import both.
- A new example under `examples/` must fit 40x4 cells; CI records every
  example at that size.
- The README Quick start is a verbatim copy of
  `test/e2e/example_countdown_test.go`; change both together.

## Pull requests

- Add or extend a test next to the change. Renderer tests use an
  in-memory `vt.Emulator` as the `Screen`; no PTY needed.
- Add a line under "Unreleased" in [CHANGELOG.md](CHANGELOG.md).
- Keep the commit message in the imperative mood, describing the user-visible
  effect.
