# Changelog

All notable changes to this project are listed here, newest first. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Release binaries and the same notes: https://github.com/DimmKirr/termoscope/releases

## [Unreleased]

## [0.4.0] - 2026-10-13

### Added

- feat(termoscope): prefix every PNG, static SVG and recording file name with the run timestamp (`<dateTimeISO>-<name>.png`, `<dateTimeISO>-recording.svg`) — artifacts stay identifiable by run once copied out of `test/results/`; tests that read `recording.svg` by its fixed name must glob for `*-recording.svg`

### Changed

- docs(termoscope): update the SavePNG, SaveSVG, Record and ExampleRecord comments to show the prefixed names — docs match what lands on disk
- test(termoscope): glob for the prefixed artifact names in the unit, e2e and Example tests — the suite passes with the new naming

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

## [0.2.1] - 2026-10-08

### Changed

- chore(ci): build the termoscope CLI with GoReleaser (linux/darwin, amd64/arm64, tar.gz with LICENSE and README, checksums.txt) and attach the archives to the GitHub release — tagged releases ship downloadable binaries again; v0.2.0 was published without any because the previous workflow failed on an already-existing release
- chore(ci): drop the per-release example media job — the SVG, PNG and GIF assets are committed under docs/assets and still recorded on every push and PR

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

[Unreleased]: https://github.com/DimmKirr/termoscope/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/DimmKirr/termoscope/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/DimmKirr/termoscope/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/DimmKirr/termoscope/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/DimmKirr/termoscope/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/DimmKirr/termoscope/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/DimmKirr/termoscope/commits/v0.1.0
