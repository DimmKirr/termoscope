// Package gif encodes recorded terminal frames as an animated GIF.
//
// Pixels come from the raster package, so the output is fully baked: the
// embedded JetBrains Mono and Noto Emoji faces are rasterized at encode time
// and no viewer font is ever consulted. Encoding uses the standard library
// image/gif. Compared to svg the result is pixel-exact everywhere but
// fixed-resolution, and emoji are monochrome.
package gif

import (
	"bytes"
	"image"
	"image/color"
	stdgif "image/gif"
	"sort"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	xdraw "golang.org/x/image/draw"

	"github.com/dimmkirr/termoscope/raster"
	"github.com/dimmkirr/termoscope/svg"
)

// Options controls encoding. Zero values take the documented defaults.
type Options struct {
	Hold      time.Duration // how long the last frame stays before looping, default 2s
	Scale     int           // 2 (default) keeps the hi-DPI size raster uses, 1 halves it to terminal size
	MinCols   int           // minimum canvas width in cells; the canvas still grows to fit content
	MinRows   int           // minimum canvas height in cells; the canvas still grows to fit content
	MonoEmoji bool          // draw emoji with the monochrome Noto Emoji face instead of Twemoji pictures
	// Padding is the background margin added on every side, in raster (2x)
	// pixels; it is halved when Scale is 1. Default raster.Margin, one cell
	// height, matching svg. Negative disables.
	Padding int
}

// margin returns Padding in output pixels.
func (o Options) margin() int {
	if o.Padding < 0 {
		return 0
	}
	return o.Padding * o.Scale / 2
}

func (o Options) withDefaults() Options {
	if o.Hold == 0 {
		o.Hold = 2 * time.Second
	}
	if o.Scale != 1 {
		o.Scale = 2
	}
	if o.Padding == 0 {
		o.Padding = raster.Margin
	}
	return o
}

// Render encodes frames as a looping animated GIF. Each frame is shown
// until the next frame's timestamp; the last one for Hold. The canvas is
// the smallest cell grid containing every frame, at least MinCols x MinRows.
func Render(frames []svg.Frame, opts Options) ([]byte, error) {
	o := opts.withDefaults()
	frames = dropLeadingBlank(frames)
	if len(frames) == 0 {
		frames = []svg.Frame{{}}
	}
	cols, rows := svg.Bounds(frames)
	cols, rows = max(cols, o.MinCols), max(rows, o.MinRows)

	imgs := make([]*image.RGBA, len(frames))
	for i, f := range frames {
		img, err := raster.RenderWith(newFrameScreen(f, cols, rows), raster.Options{MonoEmoji: o.MonoEmoji})
		if err != nil {
			return nil, err
		}
		if o.Scale == 1 {
			img = downscale(img)
		}
		imgs[i] = img
	}
	imgs = raster.Pad(imgs, o.margin())

	pal := buildPalette(imgs)
	g := &stdgif.GIF{LoopCount: 0}
	for i, img := range imgs {
		g.Image = append(g.Image, toPaletted(img, pal))
		var d time.Duration
		if i+1 < len(frames) {
			d = frames[i+1].At - frames[i].At
		} else {
			d = o.Hold
		}
		g.Delay = append(g.Delay, max(int(d/(10*time.Millisecond)), 2))
		g.Disposal = append(g.Disposal, stdgif.DisposalNone)
	}

	var b bytes.Buffer
	if err := stdgif.EncodeAll(&b, g); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// dropLeadingBlank removes frames before the first one with any content
// and rebases timestamps so the first kept frame starts at zero. A
// recording always begins with the empty screen sampled before the program
// printed anything; keeping it would make every static GIF preview (file
// browsers, image viewers, GitHub's file page) show a blank picture. At
// least one frame is always kept.
func dropLeadingBlank(frames []svg.Frame) []svg.Frame {
	first := 0
	for first < len(frames)-1 && isBlank(frames[first]) {
		first++
	}
	if first == 0 {
		return frames
	}
	out := make([]svg.Frame, len(frames)-first)
	base := frames[first].At
	for i, f := range frames[first:] {
		f.At -= base
		out[i] = f
	}
	return out
}

func isBlank(f svg.Frame) bool {
	for y := range f.Lines {
		if f.Text(y) != "" {
			return false
		}
	}
	return true
}

// downscale halves the hi-DPI raster output to terminal size.
func downscale(src *image.RGBA) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx()/2, b.Dy()/2))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Src, nil)
	return dst
}

// buildPalette returns at most 256 colors covering every pixel of imgs.
// Terminal screens have few distinct colors, but anti-aliased glyph edges
// add many blends; exact colors are kept while they fit, then the rest are
// quantized to a 6x6x6 color cube plus the colors already chosen.
func buildPalette(imgs []*image.RGBA) color.Palette {
	count := map[color.RGBA]int{}
	for _, img := range imgs {
		b := img.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				count[img.RGBAAt(x, y)]++
			}
		}
	}
	if len(count) <= 256 {
		pal := make(color.Palette, 0, len(count))
		for c := range count {
			pal = append(pal, c)
		}
		return pal
	}
	// Keep the 40 most frequent exact colors (solid areas, pure text
	// colors) and fill the remaining 216 slots with a 6x6x6 cube that
	// approximates the anti-aliasing blends.
	type kv struct {
		c color.RGBA
		n int
	}
	kvs := make([]kv, 0, len(count))
	for c, n := range count {
		kvs = append(kvs, kv{c, n})
	}
	sort.Slice(kvs, func(i, j int) bool { return kvs[i].n > kvs[j].n })
	pal := make(color.Palette, 0, 256)
	for i := 0; i < len(kvs) && i < 40; i++ {
		pal = append(pal, kvs[i].c)
	}
	for r := 0; r < 6; r++ {
		for g := 0; g < 6; g++ {
			for bl := 0; bl < 6; bl++ {
				pal = append(pal, color.RGBA{uint8(r * 51), uint8(g * 51), uint8(bl * 51), 0xff})
			}
		}
	}
	return pal
}

// toPaletted maps img onto pal, caching nearest-color lookups because
// anti-aliased blends repeat heavily.
func toPaletted(img *image.RGBA, pal color.Palette) *image.Paletted {
	b := img.Bounds()
	out := image.NewPaletted(b, pal)
	cache := map[color.RGBA]uint8{}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.RGBAAt(x, y)
			idx, ok := cache[c]
			if !ok {
				idx = uint8(pal.Index(c))
				cache[c] = idx
			}
			out.SetColorIndex(x, y, idx)
		}
	}
	return out
}

// frameScreen adapts a recorded svg.Frame to raster.Screen. Frames store
// one entry per cell with wide cells occupying several columns, so the
// adapter maps a column back to its cell and reports continuation columns
// as zero-width cells, exactly as the emulator would.
type frameScreen struct {
	f          svg.Frame
	cols, rows int
	index      [][]int // index[y][col] = cell index in f.Lines[y], or -1 for a continuation column
}

func newFrameScreen(f svg.Frame, cols, rows int) *frameScreen {
	s := &frameScreen{f: f, cols: cols, rows: rows, index: make([][]int, rows)}
	for y := 0; y < rows; y++ {
		row := make([]int, cols)
		for i := range row {
			row[i] = -2 // past the end of the recorded row: blank
		}
		if y < len(f.Lines) {
			col := 0
			for i, c := range f.Lines[y] {
				w := max(c.Width, 1)
				for k := 0; k < w && col < cols; k++ {
					if k == 0 {
						row[col] = i
					} else {
						row[col] = -1
					}
					col++
				}
			}
		}
		s.index[y] = row
	}
	return s
}

func (s *frameScreen) Width() int  { return s.cols }
func (s *frameScreen) Height() int { return s.rows }

func (s *frameScreen) CellAt(x, y int) *uv.Cell {
	if y < 0 || y >= s.rows || x < 0 || x >= s.cols {
		return nil
	}
	switch i := s.index[y][x]; i {
	case -2:
		return &uv.Cell{Content: " ", Width: 1}
	case -1:
		return &uv.Cell{Width: 0}
	default:
		c := s.f.Lines[y][i]
		return &uv.Cell{Content: c.Content, Width: max(c.Width, 1), Style: uv.Style{Fg: c.Fg, Bg: c.Bg}}
	}
}
