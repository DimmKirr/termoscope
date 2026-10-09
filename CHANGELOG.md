# Changelog

All notable changes to this project are listed here, newest first. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Release binaries and the same notes: https://github.com/DimmKirr/termoscope/releases

## [Unreleased]

## [0.3.0] - 2026-10-09

### Removed

- refactor(svg)!: rename package svganim to svg — imports change to `github.com/dimmkirr/termoscope/svg`; the svganim import path no longer exists
- refactor(gif)!: rename package gifanim to gif — imports change to `github.com/dimmkirr/termoscope/gif`; the gifanim import path no longer exists
- feat(api)!: add termoscope.Options and change RecordWith to `RecordWith(t, tm, Options)` — tests set interval, hold, minimum canvas and emoji embedding once, and the SVG and GIF recordings always agree

  Migration: replace `RecordWith(t, tm, 40*time.Millisecond, svganim.Options{Hold: h})` with `RecordWith(t, tm, termoscope.Options{Interval: 40*time.Millisecond, Hold: h})` and change `svganim`/`gifanim` imports to `svg`/`gif`.

### Added

- feat(termoscope): add ExampleStart and ExampleRecord — pkg.go.dev and go doc show runnable usage

### Fixed

- fix(termoscope): name the Unix PTY requirement and the issue tracker in Start's pty-creation error — a failed start says why and where to report it

### Changed

- docs(termoscope): rewrite the package comment, add a README "Not yet" table of known gaps, CONTRIBUTING.md and CHANGELOG.md — readers see the scope, the gaps and where a change belongs
- docs(agents): document options.go and the RecordWith contract in AGENTS.md — no user-facing impact
- chore(ci): build and publish releases with GoReleaser instead of a hand-written matrix — release assets are the CLI archives and checksums; the example SVG, PNG and GIF recordings are no longer attached

## [0.2.0] - 2026-10-08

### Added

- The `termoscope record` CLI: SVG, PNG and GIF from any command.
- Test coverage for the root package.

### Changed

- Rename the project from termproof to termoscope.

### Fixed

- Color emoji rendered oversized in PNG and GIF.

## [0.1.1] - 2026-10-08

### Added

- Animated GIF rendering with color emoji from Twemoji.

### Fixed

- A data race in `raster.Render` under concurrent calls.

## [0.1.0] - 2026-10-08

### Added

- Initial release: headless PTY testing with PNG and animated SVG evidence.
- Emoji support in the PTY screen, PNG rasterizer and SVG recorder.

[Unreleased]: https://github.com/DimmKirr/termoscope/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/DimmKirr/termoscope/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/DimmKirr/termoscope/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/DimmKirr/termoscope/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/DimmKirr/termoscope/commits/v0.1.0
