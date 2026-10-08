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
		`font-family="JetBrains Mono, monospace"`, `@font-face`, `data:font/woff2;base64,`,
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
