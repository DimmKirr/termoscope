// Command termoscope records a program running under a headless
// pseudo-terminal and writes an animated SVG, optionally with a PNG of the
// final screen.
//
//	termoscope record -o demo.svg [-png last.png] [-cols 80] [-rows 24] \
//	    [-sample 40ms] [-hold 2s] [-timeout 2m] -- cmd args...
//
// The exit code is the child's exit code; the recording is written either
// way.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/dimmkirr/termoscope"
	"github.com/dimmkirr/termoscope/gif"
	"github.com/dimmkirr/termoscope/raster"
	"github.com/dimmkirr/termoscope/svg"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func usage(w io.Writer) {
	say(w, "usage: termoscope record [flags] -- cmd [args...]")
	say(w, "       termoscope help")
}

// run parses args and returns the process exit code.
func run(args []string, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		usage(stderr)
		return 2
	}
	switch args[0] {
	case "record":
		return record(args[1:], stderr)
	default:
		sayf(stderr, "termoscope: unknown command %q\n", args[0])
		usage(stderr)
		return 2
	}
}

func record(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("termoscope record", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("o", "recording.svg", "animated SVG output path")
	pngOut := fs.String("png", "", "also write the final screen as PNG to this path")
	gifOut := fs.String("gif", "", "also write the recording as an animated GIF to this path (pixel-exact, bundled fonts, no viewer font needed)")
	gifScale := fs.Int("gif-scale", 2, "GIF pixel scale: 2 = hi-DPI (crisp), 1 = terminal size (smaller file)")
	pad := fs.Int("padding", raster.Margin, "background margin around content in the PNG and GIF, in 2x pixels (default one cell height; -1 = none)")
	monoEmoji := fs.Bool("mono-emoji", false, "draw emoji in PNG and GIF with the monochrome Noto Emoji face instead of color Twemoji pictures")
	cols := fs.Int("cols", 80, "terminal width in cells")
	rows := fs.Int("rows", 24, "terminal height in cells")
	sample := fs.Duration("sample", 40*time.Millisecond, "screen sampling interval")
	hold := fs.Duration("hold", 2*time.Second, "how long the last frame stays before the loop restarts")
	timeout := fs.Duration("timeout", 2*time.Minute, "kill the program after this long")
	fontSize := fs.Float64("font-size", 16, "SVG font size in px")
	embedEmoji := fs.Bool("embed-emoji", false, "embed the bundled Noto Emoji face in the SVG (monochrome, +~750 KB) instead of relying on the viewer's color emoji font")
	minCols := fs.Int("min-cols", 0, "minimum SVG canvas width in cells (0 = fit content)")
	minRows := fs.Int("min-rows", 0, "minimum SVG canvas height in cells (0 = fit content)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cmd := fs.Args()
	if len(cmd) == 0 {
		say(stderr, "termoscope record: missing command after flags (use -- cmd args...)")
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	tm, err := termoscope.Start(ctx, *cols, *rows, cmd[0], cmd[1:]...)
	if err != nil {
		say(stderr, "termoscope record:", err)
		return 1
	}
	defer tm.Close()

	frames := svg.Record(tm, *sample)
	exitErr := tm.Wait()

	svg := svg.Render(frames, svg.Options{Hold: *hold, FontSize: *fontSize, EmbedEmoji: *embedEmoji, MinCols: *minCols, MinRows: *minRows})
	if err := os.WriteFile(*out, svg, 0o644); err != nil {
		say(stderr, "termoscope record:", err)
		return 1
	}
	sayf(stderr, "wrote %s (%d frames)\n", *out, len(frames))
	if *gifOut != "" {
		data, err := gif.Render(frames, gif.Options{Hold: *hold, Scale: *gifScale, Padding: *pad, MinCols: *minCols, MinRows: *minRows, MonoEmoji: *monoEmoji})
		if err == nil {
			err = os.WriteFile(*gifOut, data, 0o644)
		}
		if err != nil {
			say(stderr, "termoscope record:", err)
			return 1
		}
		sayf(stderr, "wrote %s\n", *gifOut)
	}
	if *pngOut != "" {
		if err := writePNG(raster.Trim(tm, *minCols, *minRows), *pngOut, raster.Options{MonoEmoji: *monoEmoji}, *pad); err != nil {
			say(stderr, "termoscope record:", err)
			return 1
		}
		sayf(stderr, "wrote %s\n", *pngOut)
	}
	return exitCode(exitErr)
}

func writePNG(s raster.Screen, path string, o raster.Options, pad int) error {
	img, err := raster.RenderWith(s, o)
	if err != nil {
		return err
	}
	if pad > 0 {
		img = raster.Pad([]*image.RGBA{img}, pad)[0]
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return png.Encode(f, img)
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return 1
}

func say(w io.Writer, a ...any)                 { _, _ = fmt.Fprintln(w, a...) }
func sayf(w io.Writer, format string, a ...any) { _, _ = fmt.Fprintf(w, format, a...) }
