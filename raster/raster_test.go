package raster

import (
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
