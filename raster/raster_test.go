package raster

import (
	"image"
	"testing"

	"github.com/charmbracelet/x/vt"
)

func TestRender_CellGeometryAndColors(t *testing.T) {
	emu := vt.NewEmulator(6, 2)
	_, _ = emu.WriteString("\x1b[38;2;57;211;83m██\x1b[0m hi")

	img, err := Render(emu)
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	if b.Dx() != 6*CellWidth || b.Dy() != 2*CellHeight {
		t.Fatalf("bounds %v, want %dx%d", b, 6*CellWidth, 2*CellHeight)
	}
	r, g, bl, _ := img.At(CellWidth/2, CellHeight/2).RGBA()
	if g <= r || g <= bl {
		t.Fatalf("cell (0,0) should be green, got r=%d g=%d b=%d", r, g, bl)
	}
	// An empty cell keeps the default background.
	r, g, bl, _ = img.At(5*CellWidth+CellWidth/2, CellHeight+CellHeight/2).RGBA()
	if r>>8 != 0x0d || g>>8 != 0x11 || bl>>8 != 0x17 {
		t.Fatalf("blank cell should be background, got %d %d %d", r>>8, g>>8, bl>>8)
	}
}

// TestRender_EmojiUsesFallbackFace proves a wide emoji cell gets ink from
// the bundled Noto Emoji (JetBrains Mono has no emoji glyphs) inside its
// two-cell span, and that the glyph after it is drawn at its own column.
func TestRender_EmojiUsesFallbackFace(t *testing.T) {
	emu := vt.NewEmulator(6, 1)
	_, _ = emu.WriteString("a🚀b")
	img, err := Render(emu)
	if err != nil {
		t.Fatal(err)
	}
	ink := func(x0, x1 int) int {
		n := 0
		for py := 0; py < CellHeight; py++ {
			for px := x0; px < x1; px++ {
				r, g, b, _ := img.At(px, py).RGBA()
				if r>>8 != 0x0d || g>>8 != 0x11 || b>>8 != 0x17 {
					n++
				}
			}
		}
		return n
	}
	if ink(CellWidth, 3*CellWidth) == 0 {
		t.Fatal("emoji span is blank: fallback face not used")
	}
	if ink(3*CellWidth, 4*CellWidth) == 0 {
		t.Fatal("glyph after the emoji missing from column 3")
	}
	if ink(4*CellWidth, 6*CellWidth) != 0 {
		t.Fatal("ink past the last glyph: emoji advanced the pen too far")
	}
}

// saturation returns the largest channel spread found in the rect, a
// cheap proxy for "is there color here" versus grey text/background.
func saturation(img *image.RGBA, r image.Rectangle) int {
	best := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := img.RGBAAt(x, y)
			hi := max(c.R, c.G, c.B)
			lo := min(c.R, c.G, c.B)
			best = max(best, int(hi-lo))
		}
	}
	return best
}

// TestRender_ColorEmojiByDefault proves emoji come from the Twemoji
// pictures unless MonoEmoji is set: the rocket span is colorful by default
// and grey (foreground-tinted Noto Emoji) in mono mode. It also checks a
// ZWJ family and a flag, which only resolve by whole-cluster lookup.
func TestRender_ColorEmojiByDefault(t *testing.T) {
	emu := vt.NewEmulator(10, 1)
	_, _ = emu.WriteString("🚀 👨‍👩‍👧 🇨🇿")
	colorImg, err := Render(emu)
	if err != nil {
		t.Fatal(err)
	}
	monoImg, err := RenderWith(emu, Options{MonoEmoji: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		col  int
	}{{"rocket", 0}, {"family", 3}, {"flag", 6}} {
		r := image.Rect(tc.col*CellWidth, 0, (tc.col+2)*CellWidth, CellHeight)
		if s := saturation(colorImg, r); s < 0x60 {
			t.Errorf("%s: default render should be colorful, max channel spread %d", tc.name, s)
		}
	}
	if s := saturation(monoImg, image.Rect(0, 0, 2*CellWidth, CellHeight)); s > 0x20 {
		t.Errorf("mono render should be grey, max channel spread %d", s)
	}
}

// TestRender_TextSymbolsStayText proves symbols JetBrains Mono owns are
// not swapped for pictures just because Twemoji also has them.
func TestRender_TextSymbolsStayText(t *testing.T) {
	emu := vt.NewEmulator(6, 1)
	_, _ = emu.WriteString("#*0©")
	img, err := Render(emu)
	if err != nil {
		t.Fatal(err)
	}
	if s := saturation(img, img.Bounds()); s > 0x20 {
		t.Errorf("plain symbols must render as grey text, max channel spread %d", s)
	}
}

func TestPad_AddsExactMarginOnEverySide(t *testing.T) {
	emu := vt.NewEmulator(3, 1)
	_, _ = emu.WriteString("█")
	img, err := Render(emu)
	if err != nil {
		t.Fatal(err)
	}
	out := Pad([]*image.RGBA{img}, 4)[0]
	b := out.Bounds()
	if b.Dx() != 3*CellWidth+8 || b.Dy() != CellHeight+8 {
		t.Fatalf("bounds %v, want %dx%d", b, 3*CellWidth+8, CellHeight+8)
	}
	if out.RGBAAt(0, 0) != Background || out.RGBAAt(3, 3) != Background || out.RGBAAt(b.Max.X-1, b.Max.Y-1) != Background {
		t.Error("margin must be background")
	}
	if out.RGBAAt(4, 4) == Background {
		t.Error("content must start right after the margin")
	}
	if got := Pad([]*image.RGBA{img}, 0); got[0] != img {
		t.Error("zero margin must return the input")
	}
}

func TestTrim_CropsToContentCells(t *testing.T) {
	emu := vt.NewEmulator(40, 4)
	_, _ = emu.WriteString("Hi 🚀\r\n\r\n  \x1b[48;2;1;2;3m \x1b[0m") // bg-only cell on row 2
	s := Trim(emu, 0, 0)
	if s.Width() != 5 || s.Height() != 3 {
		t.Errorf("trimmed to %dx%d, want 5x3 (wide rocket ends at col 5, bg cell on row 3)", s.Width(), s.Height())
	}
	s = Trim(emu, 20, 4)
	if s.Width() != 20 || s.Height() != 4 {
		t.Errorf("min size not honored: %dx%d", s.Width(), s.Height())
	}
	s = Trim(emu, 99, 99)
	if s.Width() != 40 || s.Height() != 4 {
		t.Errorf("must not exceed the screen: %dx%d", s.Width(), s.Height())
	}
	img, err := Render(Trim(vt.NewEmulator(10, 3), 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != CellWidth || b.Dy() != CellHeight {
		t.Errorf("empty screen trims to one cell, got %v", b)
	}
}

// inkBounds returns the bounding box of non-background pixels inside r.
func inkBounds(img *image.RGBA, r image.Rectangle) image.Rectangle {
	var b image.Rectangle
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := img.RGBAAt(x, y)
			if c.R != 0x0d || c.G != 0x11 || c.B != 0x17 {
				b = b.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return b
}

// TestRender_ColorEmojiIsEmSized proves a Twemoji picture is drawn at the
// font's em size (FontSize px), like text and like the SVG's emoji text at
// font-size 16, not stretched to the two-cell box. ✅ fills its Twemoji
// canvas edge to edge, so its ink box is the picture box.
func TestRender_ColorEmojiIsEmSized(t *testing.T) {
	emu := vt.NewEmulator(4, 1)
	_, _ = emu.WriteString("✅")
	img, err := Render(emu)
	if err != nil {
		t.Fatal(err)
	}
	span := image.Rect(0, 0, 2*CellWidth, CellHeight)
	ink := inkBounds(img, span)
	if ink.Dx() > FontSize || ink.Dy() > FontSize {
		t.Fatalf("emoji ink %dx%d exceeds the %dpx em", ink.Dx(), ink.Dy(), FontSize)
	}
	if ink.Dx() < FontSize-6 || ink.Dy() < FontSize-6 {
		t.Fatalf("emoji ink %dx%d is far smaller than the %dpx em", ink.Dx(), ink.Dy(), FontSize)
	}
	// Centered in the span: left and right slack within a pixel of each other,
	// same for top and bottom.
	if l, r := ink.Min.X, span.Max.X-ink.Max.X; abs(l-r) > 1 {
		t.Errorf("not horizontally centered: left %d, right %d", l, r)
	}
	if tp, bt := ink.Min.Y, span.Max.Y-ink.Max.Y; abs(tp-bt) > 1 {
		t.Errorf("not vertically centered: top %d, bottom %d", tp, bt)
	}
}

func abs(i int) int {
	if i < 0 {
		return -i
	}
	return i
}
