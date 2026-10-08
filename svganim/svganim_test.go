package svganim

import (
	"bytes"
	"encoding/xml"
	"image/color"
	"strings"
	"testing"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
)

// fakeSource is a thread-safe emulator written to from another goroutine,
// mirroring how pty.Terminal pumps PTY output while Record samples it.
type fakeSource struct {
	emu  *vt.SafeEmulator
	done chan struct{}
}

func (f *fakeSource) Width() int               { return f.emu.Width() }
func (f *fakeSource) Height() int              { return f.emu.Height() }
func (f *fakeSource) CellAt(x, y int) *uv.Cell { return f.emu.CellAt(x, y) }
func (f *fakeSource) Done() <-chan struct{}    { return f.done }

func green(s string) string { return "\x1b[38;2;57;211;83m" + s + "\x1b[0m" }

func TestSnapshot_CapturesTextTilesAndColors(t *testing.T) {
	emu := vt.NewEmulator(20, 3)
	_, _ = emu.WriteString("Init  " + green("▄ ▄") + "\r\n─────────\r\n" + green("▄") + " Processing")
	fr := Snapshot(emu, 0)
	if fr.Rows() != 3 {
		t.Fatalf("rows %d", fr.Rows())
	}
	if got := fr.Text(0); got != "Init  ▄ ▄" {
		t.Fatalf("line 0 = %q", got)
	}
	c := fr.Lines[0][6]
	if c.Content != "▄" {
		t.Fatalf("expected tile at col 6, got %q", c.Content)
	}
	r, g, b, _ := c.Fg.RGBA()
	if g <= r || g <= b {
		t.Fatalf("tile must keep its color, got %d %d %d", r>>8, g>>8, b>>8)
	}
	if fr.Lines[0][0].Fg != nil {
		t.Fatal("uncolored text must have nil Fg")
	}
}

func TestRecord_DedupesIdenticalFramesAndStopsOnDone(t *testing.T) {
	emu := vt.NewSafeEmulator(10, 1)
	src := &fakeSource{emu: emu, done: make(chan struct{})}
	go func() {
		time.Sleep(30 * time.Millisecond)
		_, _ = emu.Write([]byte("a"))
		time.Sleep(30 * time.Millisecond)
		_, _ = emu.Write([]byte("b"))
		time.Sleep(30 * time.Millisecond)
		close(src.done)
	}()
	frames := Record(src, 5*time.Millisecond)
	if len(frames) != 3 { // blank, "a", "ab"
		for _, f := range frames {
			t.Logf("%v %q", f.At, f.Text(0))
		}
		t.Fatalf("want 3 distinct frames, got %d", len(frames))
	}
	if frames[0].At != 0 || frames[1].At <= frames[0].At || frames[2].At <= frames[1].At {
		t.Fatalf("timestamps must increase from zero: %v %v %v", frames[0].At, frames[1].At, frames[2].At)
	}
	if frames[2].Text(0) != "ab" {
		t.Fatalf("last frame %q", frames[2].Text(0))
	}
}

func TestRender_ProducesAnimatedSVG(t *testing.T) {
	emu := vt.NewEmulator(30, 4)
	_, _ = emu.WriteString("Init  " + green("▄ ▄") + "\r\n─────────\r\n" + green("▄") + " Done <ok> & fine")
	f0 := Snapshot(emu, 0)
	_, _ = emu.WriteString("\r\n")
	_, _ = emu.WriteString("more")
	f1 := Snapshot(emu, 500*time.Millisecond)
	svg := Render([]Frame{f0, f1}, Options{})

	if err := xml.Unmarshal(svg, new(struct{})); err != nil {
		t.Fatalf("invalid XML: %v\n%s", err, svg)
	}
	s := string(svg)
	for _, want := range []string{
		`<svg`, `xmlns="http://www.w3.org/2000/svg"`,
		`@keyframes`, `class="f"`, `id="f0"`, `id="f1"`,
		`fill="#39d353"`, `rx="`, `<rect`, // tiles as rounded rects
		`textLength=`, `lengthAdjust="spacing"`,
		`Done &lt;ok&gt; &amp; fine`, // escaped text
		`font-family="JetBrains Mono, Twemoji Mozilla, Apple Color Emoji, Segoe UI Emoji, Segoe UI Symbol, Noto Color Emoji, EmojiOne Color, Android Emoji, monospace"`, `@font-face`, `data:font/woff2;base64,`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Count(s, `class="f"`) != 2 {
		t.Fatalf("want 2 frame groups, got %d", strings.Count(s, `class="f"`))
	}
	if strings.Contains(s, "▄") {
		t.Fatal("block glyphs must be drawn as rects, not text")
	}
	// Rule line drawn as a rect, not box-drawing text.
	if strings.Contains(s, "─") {
		t.Fatal("rule must be drawn as a rect, not text")
	}
}

func TestRender_DurationCoversLastFramePlusHold(t *testing.T) {
	emu := vt.NewEmulator(5, 1)
	_, _ = emu.WriteString("x")
	frames := []Frame{Snapshot(emu, 0), Snapshot(emu, 2*time.Second)}
	svg := Render(frames, Options{Hold: 3 * time.Second})
	if !bytes.Contains(svg, []byte("5s steps(1, end) infinite")) {
		t.Fatalf("animation duration must be last.At + Hold:\n%s", svg)
	}
}

func TestRenderStatic_SingleFrameNoAnimation(t *testing.T) {
	emu := vt.NewEmulator(5, 1)
	_, _ = emu.WriteString("x")
	svg := RenderStatic(Snapshot(emu, 0), Options{})
	if bytes.Contains(svg, []byte("@keyframes")) || !bytes.Contains(svg, []byte(">x</text>")) {
		t.Fatalf("static render must have no animation and keep text:\n%s", svg)
	}
}

func TestFrame_Bounds_TrimsToContent(t *testing.T) {
	emu := vt.NewEmulator(80, 24)
	_, _ = emu.WriteString("ab\r\n\r\ncd")
	fr := Snapshot(emu, 0)
	w, h := Bounds([]Frame{fr})
	if w != 2 || h != 3 {
		t.Fatalf("bounds %dx%d, want 2x3", w, h)
	}
}

var _ color.Color = color.RGBA{}

// TestRenderStatic_WideCellsKeepColumns proves a two-column emoji is drawn
// centered in its span and the text after it starts at its true column,
// rather than one column early because continuation cells are dropped.
func TestRenderStatic_WideCellsKeepColumns(t *testing.T) {
	emu := vt.NewEmulator(10, 1)
	_, _ = emu.WriteString("Hi 🚀 go")
	o := Options{}.withDefaults()
	cw, ch := o.cell()
	svg := string(RenderStatic(Snapshot(emu, 0), Options{NoEmbed: true}))

	// H=0 i=1 ' '=2 🚀=3-4 ' '=5 g=6 o=7
	rocket := `<text x="` + num(o.Padding+4*cw) + `" y="` + num(o.Padding+ch/2+o.tile()/2) + `" fill="#c9d1d9" text-anchor="middle">🚀</text>`
	if !strings.Contains(svg, rocket) {
		t.Errorf("rocket not centered on column 4:\n%s", svg)
	}
	if !strings.Contains(svg, `<text x="`+num(o.Padding+6*cw)+`" y=`) {
		t.Errorf("text after the emoji should start at column 6:\n%s", svg)
	}
	if !strings.Contains(svg, `textLength="`+num(2*cw)+`"`) {
		t.Errorf("'go' should be two columns wide:\n%s", svg)
	}
	if w, _ := Bounds([]Frame{Snapshot(emu, 0)}); w != 8 {
		t.Errorf("Bounds width = %d, want 8 columns", w)
	}
}

func TestRenderStatic_EmbedEmojiAddsFontFace(t *testing.T) {
	emu := vt.NewEmulator(4, 1)
	_, _ = emu.WriteString("🚀")
	f := Snapshot(emu, 0)
	plain := string(RenderStatic(f, Options{NoEmbed: true}))
	if strings.Contains(plain, "Noto Emoji") {
		t.Fatal("emoji font must not be embedded by default")
	}
	if !strings.Contains(plain, "Apple Color Emoji") {
		t.Fatal("font stack must fall back to the viewer's color emoji fonts")
	}
	with := string(RenderStatic(f, Options{NoEmbed: true, EmbedEmoji: true}))
	if !strings.Contains(with, "@font-face{font-family:'Noto Emoji';src:url(data:font/woff;base64,") {
		t.Fatal("EmbedEmoji should add a Noto Emoji @font-face")
	}
	if !strings.Contains(with, `font-family="JetBrains Mono, Noto Emoji, Twemoji Mozilla, Apple Color Emoji`) {
		t.Fatal("embedded emoji face should sit right after the text face in the stack")
	}
}

// TestRender_MinCanvasGrowsBackground proves MinCols/MinRows enlarge the
// canvas and its background beyond the content, while content larger than
// the minimum still wins.
func TestRender_MinCanvasGrowsBackground(t *testing.T) {
	emu := vt.NewEmulator(10, 1)
	_, _ = emu.WriteString("hi")
	f := Snapshot(emu, 0)
	o := Options{NoEmbed: true, MinCols: 20, MinRows: 4}.withDefaults()
	cw, ch := o.cell()
	svg := string(RenderStatic(f, o))
	w, h := num(20*cw+2*o.Padding), num(4*ch+2*o.Padding)
	if !strings.Contains(svg, `width="`+w+`" height="`+h+`"`) {
		t.Errorf("canvas should be 20x4 cells (%sx%s px):\n%s", w, h, svg[:200])
	}
	if !strings.Contains(svg, `<rect width="100%" height="100%"`) {
		t.Error("background must cover the whole canvas")
	}
	small := string(RenderStatic(f, Options{NoEmbed: true, MinCols: 1, MinRows: 1}))
	if !strings.Contains(small, `width="`+num(2*cw+2*o.Padding)+`"`) {
		t.Error("content wider than MinCols must still size the canvas")
	}
}
