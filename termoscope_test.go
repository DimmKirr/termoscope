package termoscope_test

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dimmkirr/termoscope"
)

const green = "\x1b[38;2;57;211;83m"

// sh runs a shell snippet under the harness so tests need no fixture binary.
func sh(t *testing.T, cols, rows int, script string) (*termoscope.Terminal, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	tm, err := termoscope.Start(ctx, cols, rows, "sh", "-c", script)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tm.Close)
	return tm, ctx
}

func TestStart_ScreenAndCells(t *testing.T) {
	tm, ctx := sh(t, 20, 3, `printf 'hi \033[38;2;57;211;83mok\033[0m\n'`)
	if _, err := tm.WaitFor(ctx, "ok"); err != nil {
		t.Fatal(err)
	}
	if err := tm.Wait(); err != nil {
		t.Fatalf("exit: %v", err)
	}
	if got := tm.Line(0); got != "hi ok" {
		t.Fatalf("line 0 = %q", got)
	}
	if w, h := tm.Width(), tm.Height(); w != 20 || h != 3 {
		t.Fatalf("size %dx%d", w, h)
	}
	c := tm.CellAt(3, 0)
	if c == nil || c.Content != "o" || c.Style.Fg == nil {
		t.Fatalf("cell (3,0) should be a green 'o', got %+v", c)
	}
	r, g, b, _ := c.Style.Fg.RGBA()
	if g <= r || g <= b {
		t.Fatalf("expected green, got %d %d %d", r>>8, g>>8, b>>8)
	}
	if strings.Contains(tm.Screen(), "\x1b") {
		t.Fatal("Screen must strip escape codes")
	}
}

func TestWait_ReportsNonZeroExit(t *testing.T) {
	tm, _ := sh(t, 10, 2, `exit 3`)
	if err := tm.Wait(); err == nil {
		t.Fatal("expected exit error")
	}
	select {
	case <-tm.Done():
	default:
		t.Fatal("Done must be closed after Wait returns")
	}
}

func TestWaitUntil_TimesOutWithScreen(t *testing.T) {
	tm, _ := sh(t, 10, 2, `printf 'never'; sleep 5`)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	err := tm.WaitUntil(ctx, func(tm *termoscope.Terminal) bool { return strings.Contains(tm.Screen(), "yes") })
	if err == nil || !strings.Contains(err.Error(), "never") {
		t.Fatalf("timeout error must include the screen, got %v", err)
	}
}

func TestStripANSI(t *testing.T) {
	if got := termoscope.StripANSI(green + "x\x1b[0m\x1b[2K"); got != "x" {
		t.Fatalf("got %q", got)
	}
}

func TestArtifacts_WrittenUnderResultsDir(t *testing.T) {
	root := t.TempDir()
	termoscope.SetResultsRoot(root)
	t.Cleanup(func() { termoscope.SetResultsRoot("") })

	tm, ctx := sh(t, 12, 2, `printf '\033[38;2;57;211;83m\342\226\204\033[0m done\n'; sleep 0.1`)
	termoscope.Record(t, tm)
	if _, err := tm.WaitFor(ctx, "done"); err != nil {
		t.Fatal(err)
	}
	pngPath := termoscope.SavePNG(t, tm, "final")
	svgPath := termoscope.SaveSVG(t, tm, "final")
	_ = tm.Wait()

	dir := termoscope.Dir(t)
	want := filepath.Join(root, "test", "results")
	if !strings.HasPrefix(dir, want) || !strings.HasSuffix(dir, "-TestArtifacts_WrittenUnderResultsDir") {
		t.Fatalf("dir %s must be under %s and end with the test name", dir, want)
	}
	if filepath.Dir(pngPath) != dir || filepath.Dir(svgPath) != dir {
		t.Fatalf("artifacts must share Dir: %s %s %s", dir, pngPath, svgPath)
	}
	f, err := os.Open(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if _, err := png.Decode(f); err != nil {
		t.Fatalf("png: %v", err)
	}
	svg, err := os.ReadFile(svgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(svg), `fill="#39d353"`) || !strings.Contains(string(svg), "done") {
		t.Fatalf("svg must hold the green tile and text:\n%s", svg)
	}
}

// TestRecord_WritesRecordingOnCleanup runs the recorded test as a subtest
// so the parent can inspect what cleanup left behind.
func TestRecord_WritesRecordingOnCleanup(t *testing.T) {
	root := t.TempDir()
	termoscope.SetResultsRoot(root)
	t.Cleanup(func() { termoscope.SetResultsRoot("") })

	var dir string
	t.Run("inner", func(t *testing.T) {
		tm, ctx := sh(t, 10, 2, `printf 'a'; sleep 0.1; printf 'b'; sleep 0.1`)
		termoscope.Record(t, tm)
		dir = termoscope.Dir(t)
		if _, err := tm.WaitFor(ctx, "ab"); err != nil {
			t.Fatal(err)
		}
		_ = tm.Wait()
	})
	data, err := os.ReadFile(filepath.Join(dir, "recording.svg"))
	if err != nil {
		t.Fatalf("recording.svg missing: %v", err)
	}
	if n := strings.Count(string(data), `class="f"`); n < 2 {
		t.Fatalf("recording should hold at least 2 frames, got %d", n)
	}
}
