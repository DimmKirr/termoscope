package gif

import (
	"bytes"
	stdgif "image/gif"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"

	"github.com/dimmkirr/termoscope/raster"
	"github.com/dimmkirr/termoscope/svg"
)

func green(s string) string { return "\x1b[38;2;57;211;83m" + s + "\x1b[0m" }

func frames(t *testing.T) []svg.Frame {
	t.Helper()
	emu := vt.NewEmulator(20, 2)
	_, _ = emu.WriteString("Init  " + green("▄ ▄ ▄") + "\r\n🚀 go")
	f0 := svg.Snapshot(emu, 0)
	_, _ = emu.WriteString("\r\x1b[K" + green("✅ done"))
	f1 := svg.Snapshot(emu, 300*time.Millisecond)
	return []svg.Frame{f0, f1}
}

// decode returns the GIF and fails the test on a malformed stream.
func decode(t *testing.T, data []byte) *stdgif.GIF {
	t.Helper()
	g, err := stdgif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestRender_FramesDelaysAndLoop(t *testing.T) {
	data, err := Render(frames(t), Options{Hold: time.Second, Padding: -1})
	if err != nil {
		t.Fatal(err)
	}
	g := decode(t, data)
	if len(g.Image) != 2 {
		t.Fatalf("frames = %d, want 2", len(g.Image))
	}
	if g.LoopCount != 0 {
		t.Errorf("LoopCount = %d, want 0 (loop forever)", g.LoopCount)
	}
	// 300ms until frame 2, then the 1s hold; delays are centiseconds.
	if g.Delay[0] != 30 || g.Delay[1] != 100 {
		t.Errorf("delays = %v, want [30 100]", g.Delay)
	}
	// Default scale 2: the raster cell size. Content is 11 cols x 2 rows.
	b := g.Image[0].Bounds()
	if b.Dx() != 11*raster.CellWidth || b.Dy() != 2*raster.CellHeight {
		t.Errorf("bounds %v, want %dx%d", b, 11*raster.CellWidth, 2*raster.CellHeight)
	}
}

func TestRender_Scale1AndMinCanvas(t *testing.T) {
	data, err := Render(frames(t), Options{Scale: 1, MinCols: 20, MinRows: 4, Padding: -1})
	if err != nil {
		t.Fatal(err)
	}
	g := decode(t, data)
	b := g.Image[0].Bounds()
	if b.Dx() != 20*raster.CellWidth/2 || b.Dy() != 4*raster.CellHeight/2 {
		t.Errorf("bounds %v, want %dx%d", b, 20*raster.CellWidth/2, 4*raster.CellHeight/2)
	}
}

// TestRender_PixelsCarryColorAndEmoji proves the frames are really drawn:
// a green tile stays green after palette mapping, the emoji span has ink
// from the bundled Noto Emoji, and the second frame differs from the first.
func TestRender_PixelsCarryColorAndEmoji(t *testing.T) {
	data, err := Render(frames(t), Options{Scale: 2, Padding: -1})
	if err != nil {
		t.Fatal(err)
	}
	g := decode(t, data)
	f0 := g.Image[0]
	cw, ch := raster.CellWidth, raster.CellHeight

	// ▄ is a lower-half block: sample the lower half of the cell.
	r, gr, bl, _ := f0.At(6*cw+cw/2, 3*ch/4).RGBA()
	if gr <= r || gr <= bl {
		t.Errorf("tile at col 6 should be green, got r=%d g=%d b=%d", r>>8, gr>>8, bl>>8)
	}
	ink := 0
	for y := ch; y < 2*ch; y++ {
		for x := 0; x < 2*cw; x++ {
			r, gr, bl, _ := f0.At(x, y).RGBA()
			if r>>8 != 0x0d || gr>>8 != 0x11 || bl>>8 != 0x17 {
				ink++
			}
		}
	}
	if ink == 0 {
		t.Error("rocket span is blank: emoji face not used")
	}
	same := true
	f1 := g.Image[1]
	for y := ch; y < 2*ch && same; y++ {
		for x := 0; x < 10*cw; x++ {
			if f0.ColorIndexAt(x, y) != f1.ColorIndexAt(x, y) {
				same = false
				break
			}
		}
	}
	if same {
		t.Error("second frame identical to first on the changed row")
	}
}

func TestBuildPalette_FitsIn256(t *testing.T) {
	// Many fg colors force the quantized path.
	emu := vt.NewEmulator(40, 4)
	for i := 0; i < 40; i++ {
		_, _ = emu.WriteString("\x1b[38;2;" + itoa(i*6) + ";" + itoa(255-i*6) + ";" + itoa(i*3) + "mW")
	}
	data, err := Render([]svg.Frame{svg.Snapshot(emu, 0)}, Options{Scale: 2})
	if err != nil {
		t.Fatal(err)
	}
	g := decode(t, data)
	if n := len(g.Image[0].Palette); n > 256 {
		t.Fatalf("palette %d > 256", n)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestFrameScreen_MapsWideCellsToColumns(t *testing.T) {
	emu := vt.NewEmulator(10, 1)
	_, _ = emu.WriteString("a🚀b")
	s := newFrameScreen(svg.Snapshot(emu, 0), 6, 1)
	if c := s.CellAt(1, 0); c == nil || c.Content != "🚀" || c.Width != 2 {
		t.Errorf("col 1 = %+v, want 2-wide rocket", c)
	}
	if c := s.CellAt(2, 0); c == nil || c.Width != 0 {
		t.Errorf("col 2 = %+v, want zero-width continuation", c)
	}
	if c := s.CellAt(3, 0); c == nil || c.Content != "b" {
		t.Errorf("col 3 = %+v, want b", c)
	}
	if c := s.CellAt(5, 0); c == nil || c.Content != " " {
		t.Errorf("col 5 = %+v, want blank padding", c)
	}
	if s.CellAt(6, 0) != nil || s.CellAt(0, 1) != nil {
		t.Error("out-of-range cells must be nil")
	}
}

// TestRender_PadsOneCellHeightByDefault proves the GIF canvas is the
// content cells plus raster.Margin on every side, like the SVG, that the
// margin halves at Scale 1, and that a negative Padding disables it.
func TestRender_PadsOneCellHeightByDefault(t *testing.T) {
	emu := vt.NewEmulator(3, 1)
	_, _ = emu.WriteString("█")
	fr := []svg.Frame{svg.Snapshot(emu, 0)}

	data, err := Render(fr, Options{Scale: 2})
	if err != nil {
		t.Fatal(err)
	}
	img := decode(t, data).Image[0]
	b := img.Bounds()
	m := raster.Margin
	if b.Dx() != raster.CellWidth+2*m || b.Dy() != raster.CellHeight+2*m {
		t.Fatalf("bounds %v, want %dx%d", b, raster.CellWidth+2*m, raster.CellHeight+2*m)
	}
	isBg := func(x, y int) bool {
		r, gr, bl, _ := img.At(x, y).RGBA()
		return r>>8 == 0x0d && gr>>8 == 0x11 && bl>>8 == 0x17
	}
	if !isBg(0, 0) || !isBg(m-1, m-1) || !isBg(b.Max.X-1, b.Max.Y-1) {
		t.Error("margin pixels should be background")
	}
	if isBg(m, m) {
		t.Error("block should start right after the margin")
	}

	data, err = Render(fr, Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	if b := decode(t, data).Image[0].Bounds(); b.Dx() != raster.CellWidth/2+2*(m/2) || b.Dy() != raster.CellHeight/2+2*(m/2) {
		t.Errorf("Scale 1: bounds %v, want %dx%d", b, raster.CellWidth/2+2*(m/2), raster.CellHeight/2+2*(m/2))
	}

	data, err = Render(fr, Options{Scale: 2, Padding: -1})
	if err != nil {
		t.Fatal(err)
	}
	if b := decode(t, data).Image[0].Bounds(); b.Dx() != raster.CellWidth || b.Dy() != raster.CellHeight {
		t.Errorf("Padding -1: bounds %v, want %dx%d", b, raster.CellWidth, raster.CellHeight)
	}
}

// TestRender_DropsLeadingBlankFrames proves the blank screen sampled
// before the program printed anything is not the GIF's first frame, so
// static previews show content, and that timing is rebased accordingly.
func TestRender_DropsLeadingBlankFrames(t *testing.T) {
	empty := vt.NewEmulator(10, 1)
	full := vt.NewEmulator(10, 1)
	_, _ = full.WriteString("hi")
	fr := []svg.Frame{
		svg.Snapshot(empty, 0),
		svg.Snapshot(empty, 50*time.Millisecond),
		svg.Snapshot(full, 120*time.Millisecond),
		svg.Snapshot(full, 420*time.Millisecond),
	}
	// Frames 0-1 are blank; frame 2 becomes t=0 and frame 3 t=300ms.
	// Frames 2 and 3 are identical in content, so the delay list shows the rebase.
	g := decode(t, mustRender(t, fr, Options{Hold: time.Second, Padding: -1}))
	if len(g.Image) != 2 {
		t.Fatalf("frames = %d, want 2 (two blank frames dropped)", len(g.Image))
	}
	if g.Delay[0] != 30 || g.Delay[1] != 100 {
		t.Errorf("delays = %v, want [30 100]", g.Delay)
	}

	// An all-blank recording keeps exactly one frame.
	g = decode(t, mustRender(t, fr[:2], Options{Padding: -1}))
	if len(g.Image) != 1 {
		t.Errorf("all-blank recording: frames = %d, want 1", len(g.Image))
	}
}

func mustRender(t *testing.T, fr []svg.Frame, o Options) []byte {
	t.Helper()
	data, err := Render(fr, o)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
