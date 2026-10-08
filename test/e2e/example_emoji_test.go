package e2e

import (
	"context"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dimmkirr/termproof"
	"github.com/dimmkirr/termproof/raster"
)

func buildEmoji(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "emoji")
	cmd := exec.Command("go", "build", "-buildvcs=false", "-o", bin, "../../examples/emoji")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return bin
}

// TestExample_Emoji proves emoji survive the whole pipeline: the emulator
// gives them two-column cells, the text after them lands on the expected
// column, the PNG draws them from the bundled Noto Emoji face, and the SVG
// keeps the grid aligned.
func TestExample_Emoji(t *testing.T) {
	bin := buildEmoji(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tm, err := termproof.Start(ctx, 40, 4, bin, "-pace=120ms")
	if err != nil {
		t.Fatal(err)
	}
	termproof.Record(t, tm)

	if _, err := tm.WaitFor(ctx, "Release"); err != nil {
		t.Fatal(err)
	}
	termproof.SavePNG(t, tm, "start")

	if _, err := tm.WaitFor(ctx, "shipped"); err != nil {
		t.Fatal(err)
	}
	if err := tm.Wait(); err != nil {
		t.Fatalf("emoji example exited with %v", err)
	}
	pngPath := termproof.SavePNG(t, tm, "shipped")
	termproof.SaveSVG(t, tm, "shipped")

	// "Release " is 8 cells; the rocket occupies columns 8-9; " v1.2" starts at 10.
	if c := tm.CellAt(8, 0); c == nil || c.Content != "🚀" || c.Width != 2 {
		t.Errorf("cell (8,0) should be a 2-wide rocket, got %+v", c)
	}
	if c := tm.CellAt(11, 0); c == nil || c.Content != "v" {
		t.Errorf("text after the emoji should start at column 11, got %+v", c)
	}
	for y := 1; y <= 2; y++ {
		if c := tm.CellAt(0, y); c == nil || c.Content != "✅" || c.Width != 2 || !isGreen(c.Style.Fg) {
			t.Errorf("cell (0,%d) should be a green check, got %+v", y, c)
		}
	}
	if got := tm.Line(3); got != "🎉 shipped" {
		t.Errorf("line 3 = %q", got)
	}

	// The PNG is cropped to content and padded by raster.Margin, so the
	// rocket's two-cell span starts at column 8 past the margin. JetBrains
	// Mono has no emoji glyphs, so this only passes when Noto Emoji is used.
	f, err := os.Open(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	ink := 0
	for py := raster.Margin; py < raster.Margin+raster.CellHeight; py++ {
		for px := raster.Margin + 8*raster.CellWidth; px < raster.Margin+10*raster.CellWidth; px++ {
			r, g, b, _ := img.At(px, py).RGBA()
			if r>>8 != 0x0d || g>>8 != 0x11 || b>>8 != 0x17 {
				ink++
			}
		}
	}
	if ink == 0 {
		t.Error("rocket cells are blank in the PNG: emoji font not used")
	}

	dir := termproof.Dir(t)
	for _, name := range []string{"start.png", "shipped.png", "shipped.svg"} {
		st, err := os.Stat(filepath.Join(dir, name))
		if err != nil || st.Size() == 0 {
			t.Errorf("artifact %s missing or empty: %v", name, err)
		}
	}
}
