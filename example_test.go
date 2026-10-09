package termoscope_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dimmkirr/termoscope"
)

// ExampleStart drives a program outside go test: start it in a 40x4 PTY,
// wait for a screen state, read cells, and wait for exit. The renderer
// packages raster, svg and gif turn the same *Terminal into images.
func ExampleStart() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tm, err := termoscope.Start(ctx, 40, 4, "./countdown", "-pace=120ms")
	if err != nil {
		panic(err)
	}
	defer tm.Close()

	if _, err := tm.WaitFor(ctx, "Liftoff"); err != nil {
		panic(err) // the error carries the current screen
	}
	if err := tm.Wait(); err != nil {
		panic(err)
	}
	if c := tm.CellAt(11, 0); c != nil {
		fmt.Println(c.Content, c.Style.Fg)
	}
	fmt.Println(tm.Line(1))
}

// ExampleRecord is the test flow, a copy of test/e2e/example_countdown_test.go
// in the repository: the body of a TestXxx that records every frame and
// leaves start.png, liftoff.png, liftoff.svg, recording.svg and
// recording.gif under test/results/<dateTimeISO>-<testName>/.
func ExampleRecord() {
	testCountdown := func(t *testing.T) {
		bin := "./countdown" // normally built into t.TempDir() with go build
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		tm, err := termoscope.Start(ctx, 40, 4, bin, "-pace=120ms")
		if err != nil {
			t.Fatal(err)
		}
		termoscope.Record(t, tm) // recording.svg and recording.gif written on cleanup

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
			if c == nil || c.Content != "▄" {
				t.Errorf("cell (%d,0) should be a tile, got %+v", x, c)
			}
		}
	}
	_ = testCountdown
}
