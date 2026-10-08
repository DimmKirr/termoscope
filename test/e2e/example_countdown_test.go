// Package e2e is the worked example for termproof: build a binary, run it
// under a headless PTY, wait for screen states, assert on cells, and leave
// PNG, SVG and an animated recording under test/results/<ts>-<test>/.
package e2e

import (
	"context"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dimmkirr/termproof"
)

func buildCountdown(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "countdown")
	cmd := exec.Command("go", "build", "-buildvcs=false", "-o", bin, "../../examples/countdown")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return bin
}

func isGreen(c color.Color) bool {
	if c == nil {
		return false
	}
	r, g, b, _ := c.RGBA()
	return g > r && g > b
}

func TestExample_Countdown(t *testing.T) {
	bin := buildCountdown(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tm, err := termproof.Start(ctx, 40, 4, bin, "-pace=120ms")
	if err != nil {
		t.Fatal(err)
	}
	termproof.Record(t, tm) // recording.svg written on cleanup

	if _, err := tm.WaitFor(ctx, "Countdown"); err != nil {
		t.Fatal(err)
	}
	termproof.SavePNG(t, tm, "start")

	if _, err := tm.WaitFor(ctx, "Liftoff"); err != nil {
		t.Fatal(err)
	}
	if err := tm.Wait(); err != nil {
		t.Fatalf("countdown exited with %v", err)
	}
	termproof.SavePNG(t, tm, "liftoff")
	termproof.SaveSVG(t, tm, "liftoff")

	// Every tile on the first row is green once the countdown finishes.
	// "Countdown  " is 11 cells wide; tiles sit at 11, 13, 15.
	for _, x := range []int{11, 13, 15} {
		c := tm.CellAt(x, 0)
		if c == nil || c.Content != "▄" || !isGreen(c.Style.Fg) {
			t.Errorf("cell (%d,0) should be a green tile, got %+v", x, c)
		}
	}
	if got := tm.Line(1); got != "Liftoff" {
		t.Errorf("line 1 = %q", got)
	}

	// Artifacts exist and are not empty.
	dir := termproof.Dir(t)
	for _, name := range []string{"start.png", "liftoff.png", "liftoff.svg"} {
		st, err := os.Stat(filepath.Join(dir, name))
		if err != nil || st.Size() == 0 {
			t.Errorf("artifact %s missing or empty: %v", name, err)
		}
	}
}
