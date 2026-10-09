# Changelog

All notable changes to this project are listed here, newest first. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Release binaries and the same notes: https://github.com/DimmKirr/termoscope/releases

## [Unreleased]

### Changed

- Rename `svganim` and `gifanim` to `svg` and `gif`; add `termoscope.Options`
  and `RecordWith` so one setting drives both recordings.
- Switch release CI to GoReleaser.

### Added

- Package examples on pkg.go.dev, CONTRIBUTING.md, a "Not yet" section in
  the README and this changelog.

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

[Unreleased]: https://github.com/DimmKirr/termoscope/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/DimmKirr/termoscope/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/DimmKirr/termoscope/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/DimmKirr/termoscope/commits/v0.1.0
