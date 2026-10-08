// Command termproof records a program running under a headless
// pseudo-terminal and writes an animated SVG, optionally with a PNG of the
// final screen.
//
//	termproof record -o demo.svg [-png last.png] [-cols 80] [-rows 24] \
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
	"image/png"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/dimmkirr/termproof"
	"github.com/dimmkirr/termproof/raster"
	"github.com/dimmkirr/termproof/svganim"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func usage(w io.Writer) {
	say(w, "usage: termproof record [flags] -- cmd [args...]")
	say(w, "       termproof help")
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
		sayf(stderr, "termproof: unknown command %q\n", args[0])
		usage(stderr)
		return 2
	}
}

func record(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("termproof record", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("o", "recording.svg", "animated SVG output path")
	pngOut := fs.String("png", "", "also write the final screen as PNG to this path")
	cols := fs.Int("cols", 80, "terminal width in cells")
	rows := fs.Int("rows", 24, "terminal height in cells")
	sample := fs.Duration("sample", 40*time.Millisecond, "screen sampling interval")
	hold := fs.Duration("hold", 2*time.Second, "how long the last frame stays before the loop restarts")
	timeout := fs.Duration("timeout", 2*time.Minute, "kill the program after this long")
	fontSize := fs.Float64("font-size", 16, "SVG font size in px")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cmd := fs.Args()
	if len(cmd) == 0 {
		say(stderr, "termproof record: missing command after flags (use -- cmd args...)")
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	tm, err := termproof.Start(ctx, *cols, *rows, cmd[0], cmd[1:]...)
	if err != nil {
		say(stderr, "termproof record:", err)
		return 1
	}
	defer tm.Close()

	frames := svganim.Record(tm, *sample)
	exitErr := tm.Wait()

	svg := svganim.Render(frames, svganim.Options{Hold: *hold, FontSize: *fontSize})
	if err := os.WriteFile(*out, svg, 0o644); err != nil {
		say(stderr, "termproof record:", err)
		return 1
	}
	sayf(stderr, "wrote %s (%d frames)\n", *out, len(frames))
	if *pngOut != "" {
		if err := writePNG(tm, *pngOut); err != nil {
			say(stderr, "termproof record:", err)
			return 1
		}
		sayf(stderr, "wrote %s\n", *pngOut)
	}
	return exitCode(exitErr)
}

func writePNG(s raster.Screen, path string) error {
	img, err := raster.Render(s)
	if err != nil {
		return err
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
