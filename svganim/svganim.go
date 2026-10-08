// Package svganim records a headless terminal screen over time and renders
// the frames as a self-contained animated SVG suitable for README files.
//
// Geometry derives from the embedded font: a cell is one glyph advance wide
// and two tall, block glyphs (▄ ▀ █) become cap-height squares centered in
// their cell, the rule (─) becomes a thin bar, and text keeps its natural
// spacing centered on the same line. JetBrains Mono is embedded as a data URI so
// the animation shows the same typeface everywhere. Frames are switched with
// a CSS keyframe animation, which GitHub renders inside <img>.
package svganim

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"image/color"
	"strings"
	"time"

	uv "github.com/charmbracelet/ultraviolet"

	"github.com/dimmkirr/termproof/internal/fonts"
)

// Source is a terminal screen whose process signals completion via Done.
type Source interface {
	Screen
	Done() <-chan struct{}
}

// Screen is a readable cell grid.
type Screen interface {
	Width() int
	Height() int
	CellAt(x, y int) *uv.Cell
}

// Cell is one captured screen cell. Width is the number of columns the cell
// occupies: 1 for ordinary text, 2 for emoji and other wide glyphs.
type Cell struct {
	Content string
	Fg      color.Color // nil means the default foreground
	Width   int
}

func (c Cell) width() int { return max(c.Width, 1) }

// Frame is one captured screen with its timestamp relative to the start.
type Frame struct {
	At    time.Duration
	Lines [][]Cell
}

// Rows returns the number of captured rows.
func (f Frame) Rows() int { return len(f.Lines) }

// Text returns the right-trimmed plain text of a row.
func (f Frame) Text(y int) string {
	if y < 0 || y >= len(f.Lines) {
		return ""
	}
	return strings.TrimRight(rowText(f.Lines[y]), " ")
}

// Snapshot copies the screen into a Frame stamped at.
func Snapshot(s Screen, at time.Duration) Frame {
	w, h := s.Width(), s.Height()
	f := Frame{At: at, Lines: make([][]Cell, h)}
	for y := 0; y < h; y++ {
		row := make([]Cell, 0, w)
		for x := 0; x < w; x++ {
			c := s.CellAt(x, y)
			if c == nil || c.Width == 0 {
				continue // continuation of a wide glyph
			}
			content := c.Content
			if content == "" {
				content = " "
			}
			row = append(row, Cell{Content: content, Fg: c.Style.Fg, Width: max(c.Width, 1)})
		}
		f.Lines[y] = row
	}
	return f
}

func sameFrame(a, b Frame) bool {
	if len(a.Lines) != len(b.Lines) {
		return false
	}
	for y := range a.Lines {
		if len(a.Lines[y]) != len(b.Lines[y]) {
			return false
		}
		for x := range a.Lines[y] {
			if a.Lines[y][x].Content != b.Lines[y][x].Content || a.Lines[y][x].width() != b.Lines[y][x].width() || !colorEqual(a.Lines[y][x].Fg, b.Lines[y][x].Fg) {
				return false
			}
		}
	}
	return true
}

func colorEqual(a, b color.Color) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	r1, g1, b1, a1 := a.RGBA()
	r2, g2, b2, a2 := b.RGBA()
	return r1 == r2 && g1 == g2 && b1 == b2 && a1 == a2
}

// Record samples src every interval until Done closes, keeping only frames
// that differ from the previous one. The first frame is stamped at zero.
func Record(src Source, interval time.Duration) []Frame {
	start := time.Now()
	frames := []Frame{Snapshot(src, 0)}
	tick := time.NewTicker(interval)
	defer tick.Stop()
	capture := func() {
		f := Snapshot(src, time.Since(start))
		if !sameFrame(f, frames[len(frames)-1]) {
			frames = append(frames, f)
		}
	}
	for {
		select {
		case <-src.Done():
			// Let the final write land, then take the closing frame.
			time.Sleep(interval)
			capture()
			return frames
		case <-tick.C:
			capture()
		}
	}
}

// Bounds returns the smallest width and height (in cells) containing every
// non-blank cell across frames, at least 1x1.
func Bounds(frames []Frame) (w, h int) {
	w, h = 1, 1
	for _, f := range frames {
		for y, row := range f.Lines {
			if n := rowCols(row); n > 0 {
				if n > w {
					w = n
				}
				if y+1 > h {
					h = y + 1
				}
			}
		}
	}
	return w, h
}

func rowText(row []Cell) string {
	var b strings.Builder
	for _, c := range row {
		b.WriteString(c.Content)
	}
	return b.String()
}

// rowCols returns the number of columns up to the last non-blank cell,
// counting wide cells by their width.
func rowCols(row []Cell) int {
	cols, last := 0, 0
	for _, c := range row {
		cols += c.width()
		if c.Content != " " {
			last = cols
		}
	}
	return last
}

// Options controls rendering. Zero values take the documented defaults.
//
// Geometry derives from the font so text keeps its natural spacing: a cell
// is one glyph advance wide (0.6 em) and two advances tall (a 1.2 em line);
// a tile is a square with the font's cap height as its side, centered in
// its cell. Tiles sit in alternating cells, so the horizontal gap between
// tiles (2 advances minus a tile) equals the vertical gap (cell height minus
// a tile): both are 0.78 advance.
type Options struct {
	FontSize   float64       // px, default 16
	Padding    float64       // px around the content, default one cell height
	Radius     float64       // corner radius of tiles in px, default tile/6
	Background string        // default #0d1117
	Foreground string        // default #c9d1d9
	Hold       time.Duration // how long the last frame stays before looping, default 2s
	NoEmbed    bool          // skip the embedded font (smaller output, viewer font)
	EmbedEmoji bool          // also embed Noto Emoji (monochrome, ~750 KB) instead of relying on the viewer's color emoji font
	MinCols    int           // minimum canvas width in cells; the canvas still grows to fit content
	MinRows    int           // minimum canvas height in cells; the canvas still grows to fit content
}

// canvas returns the canvas size in cells: the content bounds, widened to
// at least MinCols x MinRows. The background rect covers the whole canvas.
func (o Options) canvas(frames []Frame) (cols, rows int) {
	cols, rows = Bounds(frames)
	return max(cols, o.MinCols), max(rows, o.MinRows)
}

// colorEmojiFonts are the color emoji faces shipped by browsers and
// operating systems, in the order the web community settled on: Firefox's
// bundled Twemoji first (it is newer than most OS fonts), then macOS/iOS,
// Windows (Segoe UI Symbol covers older Windows in monochrome), Android,
// ChromeOS and Linux, and two legacy Linux/Android names. A browser only
// consults them for glyphs the text face lacks.
var colorEmojiFonts = []string{
	"Twemoji Mozilla",
	"Apple Color Emoji",
	"Segoe UI Emoji",
	"Segoe UI Symbol",
	"Noto Color Emoji",
	"EmojiOne Color",
	"Android Emoji",
}

// fontFamily is the CSS font-family stack: the embedded text face, then the
// bundled emoji face when embedded, then the color emoji fonts, then a
// generic fallback.
func (o Options) fontFamily() string {
	fams := []string{fonts.Family}
	if o.EmbedEmoji {
		fams = append(fams, fonts.EmojiFamily)
	}
	fams = append(fams, colorEmojiFonts...)
	fams = append(fams, "monospace")
	return strings.Join(fams, ", ")
}

func (o Options) withDefaults() Options {
	if o.FontSize == 0 {
		o.FontSize = 16
	}
	if o.Padding == 0 {
		_, ch := o.cell()
		o.Padding = ch
	}
	if o.Radius == 0 {
		o.Radius = o.tile() / 6
	}
	if o.Background == "" {
		o.Background = "#0d1117"
	}
	if o.Foreground == "" {
		o.Foreground = "#c9d1d9"
	}
	if o.Hold == 0 {
		o.Hold = 2 * time.Second
	}
	return o
}

// cell geometry: one advance wide, two advances tall.
func (o Options) cell() (w, h float64) {
	a := o.FontSize * fonts.Advance
	return a, 2 * a
}

// tile is the side of a status square: the font's cap height.
func (o Options) tile() float64 { return o.FontSize * fonts.CapHeight }

func (o Options) fontSize() float64 { return o.FontSize }

// Render returns an animated SVG that plays frames at their timestamps and
// loops after Hold.
func Render(frames []Frame, opts Options) []byte {
	o := opts.withDefaults()
	if len(frames) == 0 {
		frames = []Frame{{}}
	}
	cols, rows := o.canvas(frames)
	total := frames[len(frames)-1].At + o.Hold

	var b bytes.Buffer
	writeHeader(&b, o, cols, rows)
	b.WriteString("<style>\n")
	writeFontFace(&b, o)
	fmt.Fprintf(&b, ".f{opacity:0;animation:%s steps(1, end) infinite}\n", dur(total))
	for i, f := range frames {
		startPct := pct(f.At, total)
		endPct := 100.0
		if i+1 < len(frames) {
			endPct = pct(frames[i+1].At, total)
		}
		fmt.Fprintf(&b, "#f%d{animation-name:k%d}\n", i, i)
		if startPct == 0 {
			fmt.Fprintf(&b, "@keyframes k%d{0%%{opacity:1}%s%%{opacity:0}100%%{opacity:0}}\n", i, num(endPct))
		} else {
			fmt.Fprintf(&b, "@keyframes k%d{0%%{opacity:0}%s%%{opacity:1}%s%%{opacity:0}100%%{opacity:0}}\n", i, num(startPct), num(endPct))
		}
	}
	b.WriteString("</style>\n")
	writeBackground(&b, o)
	for i, f := range frames {
		fmt.Fprintf(&b, `<g class="f" id="f%d">`+"\n", i)
		writeFrame(&b, f, o, cols, rows)
		b.WriteString("</g>\n")
	}
	b.WriteString("</svg>\n")
	return b.Bytes()
}

// RenderStatic returns a non-animated SVG of a single frame.
func RenderStatic(f Frame, opts Options) []byte {
	o := opts.withDefaults()
	cols, rows := o.canvas([]Frame{f})
	var b bytes.Buffer
	writeHeader(&b, o, cols, rows)
	if !o.NoEmbed || o.EmbedEmoji {
		b.WriteString("<style>\n")
		writeFontFace(&b, o)
		b.WriteString("</style>\n")
	}
	writeBackground(&b, o)
	writeFrame(&b, f, o, cols, rows)
	b.WriteString("</svg>\n")
	return b.Bytes()
}

func writeHeader(b *bytes.Buffer, o Options, cols, rows int) {
	cw, ch := o.cell()
	width := o.Padding*2 + float64(cols)*cw
	height := o.Padding*2 + float64(rows)*ch
	fmt.Fprintf(b, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s" font-family="%s" font-size="%s">`+"\n",
		num(width), num(height), num(width), num(height), o.fontFamily(), num(o.fontSize()))
}

func writeFontFace(b *bytes.Buffer, o Options) {
	if !o.NoEmbed {
		fmt.Fprintf(b, "@font-face{font-family:'%s';src:url(data:font/woff2;base64,%s) format('woff2')}\n",
			fonts.Family, base64.StdEncoding.EncodeToString(fonts.JetBrainsMonoWOFF2))
	}
	if o.EmbedEmoji {
		fmt.Fprintf(b, "@font-face{font-family:'%s';src:url(data:font/woff;base64,%s) format('woff')}\n",
			fonts.EmojiFamily, base64.StdEncoding.EncodeToString(fonts.NotoEmojiWOFF))
	}
}

func writeBackground(b *bytes.Buffer, o Options) {
	fmt.Fprintf(b, `<rect width="100%%" height="100%%" rx="%s" fill="%s"/>`+"\n", num(o.tile()/2), o.Background)
}

// writeFrame emits one frame's shapes: rects for block glyphs and rules,
// text runs grouped by color for everything else, and one centered <text>
// per wide glyph (emoji) so its natural advance never disturbs the
// monospace grid. Tiles and text share the cell's vertical center. The
// column (col) advances by each cell's width; the slice index (x) by one.
func writeFrame(b *bytes.Buffer, f Frame, o Options, cols, rows int) {
	cw, ch := o.cell()
	t := o.tile()
	for y := 0; y < rows && y < len(f.Lines); y++ {
		row := f.Lines[y]
		top := o.Padding + float64(y)*ch
		mid := top + ch/2
		baseline := mid + t/2 // cap height == t, centered on mid
		x, col := 0, 0
		for x < len(row) && col < cols {
			c := row[x]
			fill := o.Foreground
			if c.Fg != nil {
				fill = hex(c.Fg)
			}
			left := o.Padding + float64(col)*cw
			switch {
			case c.Content == "▄" || c.Content == "▀":
				writeRect(b, left, mid-t/2, t, t, o.Radius, fill)
				x++
				col++
			case c.Content == "█":
				writeRect(b, left, top, cw, ch, o.Radius, fill)
				x++
				col++
			case c.Content == "─":
				n := 0
				for x+n < len(row) && row[x+n].Content == "─" && colorEqual(row[x+n].Fg, c.Fg) {
					n++
				}
				writeRect(b, left, mid-0.5, float64(n)*cw, 1, 0, fill)
				x += n
				col += n
			case c.Content == " ":
				x++
				col++
			case c.width() > 1:
				span := float64(c.width()) * cw
				fmt.Fprintf(b, `<text x="%s" y="%s" fill="%s" text-anchor="middle">%s</text>`+"\n",
					num(left+span/2), num(baseline), fill, escape(c.Content))
				x++
				col += c.width()
			default:
				n, width := 0, 0
				var run strings.Builder
				for x+n < len(row) {
					nc := row[x+n]
					if isBlock(nc.Content) || nc.width() > 1 || !colorEqual(nc.Fg, c.Fg) {
						break
					}
					if nc.Content == " " && (x+n+1 >= len(row) || row[x+n+1].Content == " ") {
						break
					}
					run.WriteString(nc.Content)
					n++
					width++
				}
				text := strings.TrimRight(run.String(), " ")
				tw := width - (len(run.String()) - len(text)) // trailing spaces are one byte and one column each
				fmt.Fprintf(b, `<text x="%s" y="%s" fill="%s" textLength="%s" lengthAdjust="spacing">%s</text>`+"\n",
					num(left), num(baseline), fill, num(float64(tw)*cw), escape(text))
				x += n
				col += width
			}
		}
	}
}

func isBlock(s string) bool { return s == "▄" || s == "▀" || s == "█" || s == "─" }

func writeRect(b *bytes.Buffer, x, y, w, h, rx float64, fill string) {
	fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" rx="%s" fill="%s"/>`+"\n",
		num(x), num(y), num(w), num(h), num(rx), fill)
}

func hex(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}

func num(v float64) string {
	s := fmt.Sprintf("%.2f", v)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "" || s == "-0" {
		return "0"
	}
	return s
}

func pct(at, total time.Duration) float64 {
	if total <= 0 {
		return 0
	}
	return float64(at) / float64(total) * 100
}

func dur(d time.Duration) string { return num(d.Seconds()) + "s" }

func escape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
