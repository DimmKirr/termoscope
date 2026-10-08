// Package raster rasterizes a headless terminal screen into a hi-DPI RGBA
// image: JetBrains Mono at 2x for text, Twemoji PNGs for color emoji, and
// monochrome Noto Emoji as the fallback. It is pure: no files, no testing
// imports. Use termoscope.SavePNG to write a screenshot from a test.
package raster

import (
	"image"
	"image/color"
	"image/draw"
	"strings"
	"sync"
	"unicode/utf8"

	uv "github.com/charmbracelet/ultraviolet"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"github.com/dimmkirr/termoscope/internal/fonts"
	"github.com/dimmkirr/termoscope/internal/twemoji"
)

// Options controls rendering. The zero value is the default: color emoji.
type Options struct {
	// MonoEmoji draws emoji with the monochrome Noto Emoji face in the
	// cell's foreground color instead of the bundled Twemoji pictures.
	MonoEmoji bool
}

var (
	emojiMu    sync.Mutex
	emojiCache = map[string]*image.RGBA{} // cluster → Twemoji scaled to the emoji box
)

// Background is the canvas color behind cells without an explicit background.
var Background = color.RGBA{0x0d, 0x11, 0x17, 0xff}

// Screen is the subset of a vt emulator the rasterizer needs.
type Screen interface {
	Width() int
	Height() int
	CellAt(x, y int) *uv.Cell
}

// Cell geometry in pixels, matching svganim at its default 16 px font so a
// GIF or PNG shown at half size lines up with the SVG. FontSize is the 2x
// (hi-DPI) size of that 16 px font; a cell is one JetBrains Mono advance
// (0.6 em = 19.2, rounded to 19) wide and two advances (38.4, rounded to
// 38) tall. EmojiFontSize is chosen so a Noto Emoji glyph fits inside the
// two-cell span the emulator gives it.
const (
	FontSize      = 32
	CellWidth     = 19
	CellHeight    = 38
	EmojiFontSize = 30
)

var (
	defaultBg = Background
	defaultFg = color.RGBA{0xc9, 0xd1, 0xd9, 0xff}

	fontOnce  sync.Once
	textFont  *sfnt.Font
	emojiFont *sfnt.Font
	fontErr   error
)

// loadFonts parses the embedded fonts once. An *sfnt.Font is safe for
// concurrent use as long as each caller brings its own sfnt.Buffer.
func loadFonts() error {
	fontOnce.Do(func() {
		textFont, fontErr = opentype.Parse(fonts.JetBrainsMonoTTF)
		if fontErr != nil {
			return
		}
		emojiFont, fontErr = opentype.Parse(fonts.NotoEmojiTTF)
	})
	return fontErr
}

// faces holds the per-call font.Face values. A font.Face is NOT safe for
// concurrent use (it caches glyph rasterization), so Render builds a fresh
// pair for every call; this is cheap compared to parsing the fonts.
type faces struct {
	text, emoji font.Face
}

func newFaces() (faces, error) {
	if err := loadFonts(); err != nil {
		return faces{}, err
	}
	text, err := opentype.NewFace(textFont, &opentype.FaceOptions{Size: FontSize, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return faces{}, err
	}
	emoji, err := opentype.NewFace(emojiFont, &opentype.FaceOptions{Size: EmojiFontSize, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		return faces{}, err
	}
	return faces{text: text, emoji: emoji}, nil
}
func hasGlyph(f *sfnt.Font, buf *sfnt.Buffer, r rune) bool {
	idx, err := f.GlyphIndex(buf, r)
	return err == nil && idx != 0
}

// Render draws the screen into an RGBA image with default Options: text in
// JetBrains Mono and emoji as color Twemoji pictures.
func Render(s Screen) (*image.RGBA, error) { return RenderWith(s, Options{}) }

// RenderWith draws the screen into an RGBA image. Text is drawn with
// JetBrains Mono. A cell is treated as emoji when it is two columns wide,
// contains U+FE0F, or its first rune is missing from JetBrains Mono; such a
// cell is drawn from the Twemoji picture for its whole grapheme cluster
// (so skin tones, flags and ZWJ sequences render correctly), centered in
// the cell span. With MonoEmoji, or when Twemoji has no picture, the first
// rune is drawn with the monochrome Noto Emoji face instead.
func RenderWith(s Screen, o Options) (*image.RGBA, error) {
	f, err := newFaces()
	if err != nil {
		return nil, err
	}
	w, h := s.Width(), s.Height()
	img := image.NewRGBA(image.Rect(0, 0, w*CellWidth, h*CellHeight))
	draw.Draw(img, img.Bounds(), image.NewUniform(defaultBg), image.Point{}, draw.Src)
	ascent := f.text.Metrics().Ascent.Ceil()
	d := &font.Drawer{Dst: img, Face: f.text}
	var buf sfnt.Buffer
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := s.CellAt(x, y)
			if c == nil {
				continue
			}
			px, py := x*CellWidth, y*CellHeight
			span := CellWidth * max(c.Width, 1)
			if c.Style.Bg != nil {
				draw.Draw(img, image.Rect(px, py, px+span, py+CellHeight), image.NewUniform(c.Style.Bg), image.Point{}, draw.Src)
			}
			if strings.TrimSpace(c.Content) == "" {
				continue
			}
			fg := color.Color(defaultFg)
			if c.Style.Fg != nil {
				fg = c.Style.Fg
			}
			d.Src = image.NewUniform(fg)
			r, _ := utf8.DecodeRuneInString(c.Content)
			inText := hasGlyph(textFont, &buf, r)
			if !o.MonoEmoji && (!inText || c.Width > 1 || strings.ContainsRune(c.Content, 0xFE0F)) {
				if pic := colorEmoji(c.Content, span); pic != nil {
					at := image.Pt(px+(span-pic.Bounds().Dx())/2, py+(CellHeight-pic.Bounds().Dy())/2)
					draw.Draw(img, pic.Bounds().Add(at), pic, image.Point{}, draw.Over)
					continue
				}
			}
			if !inText && hasGlyph(emojiFont, &buf, r) {
				drawEmoji(d, f.emoji, r, px, py, span)
				continue
			}
			d.Face = f.text
			d.Dot = fixed.P(px, py+ascent)
			d.DrawString(c.Content)
		}
	}
	return img, nil
}

// colorEmoji returns the Twemoji picture for cluster scaled to an em-sized
// square (FontSize px, what the SVG's emoji text occupies at the same font
// size), shrunk only if the span is narrower than that, or nil when Twemoji
// has no picture for it. Scaled pictures are cached per cluster and side.
func colorEmoji(cluster string, span int) *image.RGBA {
	side := min(span, FontSize)
	if side <= 0 {
		return nil
	}
	ck := cluster + "\x00" + string(rune(side))
	emojiMu.Lock()
	defer emojiMu.Unlock()
	if pic, seen := emojiCache[ck]; seen {
		return pic
	}
	src, ok := twemoji.Lookup(cluster)
	var pic *image.RGBA
	if ok {
		pic = image.NewRGBA(image.Rect(0, 0, side, side))
		xdraw.CatmullRom.Scale(pic, pic.Bounds(), src, src.Bounds(), xdraw.Src, nil)
	}
	emojiCache[ck] = pic
	return pic
}

// drawEmoji draws r with the monochrome emoji face, centered both ways in
// a span px wide starting at (px, py).
func drawEmoji(d *font.Drawer, emoji font.Face, r rune, px, py, span int) {
	d.Face = emoji
	bounds, adv, _ := emoji.GlyphBounds(r)
	left := fixed.I(px) + (fixed.I(span)-adv)/2
	// bounds are relative to the dot with y growing downward, so the glyph's
	// vertical center sits at (Min.Y+Max.Y)/2 below the baseline.
	baseline := fixed.I(py) + fixed.I(CellHeight)/2 - (bounds.Min.Y+bounds.Max.Y)/2
	d.Dot = fixed.Point26_6{X: left, Y: baseline}
	d.DrawString(string(r))
}

// Margin is the default background margin around content in PNG and GIF
// output, in raster (2x) pixels: one cell height, matching svganim's
// default Padding.
const Margin = CellHeight

// Pad returns copies of imgs with margin pixels of Background added on
// every side. All images must share one size. A margin of zero or less
// returns imgs unchanged.
func Pad(imgs []*image.RGBA, margin int) []*image.RGBA {
	if margin <= 0 || len(imgs) == 0 {
		return imgs
	}
	out := make([]*image.RGBA, len(imgs))
	for i, img := range imgs {
		b := img.Bounds()
		dst := image.NewRGBA(image.Rect(0, 0, b.Dx()+2*margin, b.Dy()+2*margin))
		draw.Draw(dst, dst.Bounds(), image.NewUniform(Background), image.Point{}, draw.Src)
		draw.Draw(dst, b.Add(image.Pt(margin, margin)), img, b.Min, draw.Src)
		out[i] = dst
	}
	return out
}

// Trim returns a view of s limited to the cells that hold content: the
// smallest grid, at least 1x1 and at least minCols x minRows, containing
// every cell that has non-blank text or an explicit background. Rendering
// the view yields a canvas cropped the way svganim crops its SVG.
func Trim(s Screen, minCols, minRows int) Screen {
	cols, rows := 1, 1
	for y := 0; y < s.Height(); y++ {
		for x := 0; x < s.Width(); x++ {
			c := s.CellAt(x, y)
			if c == nil || c.Width == 0 {
				continue
			}
			if strings.TrimSpace(c.Content) == "" && c.Style.Bg == nil {
				continue
			}
			cols = max(cols, x+max(c.Width, 1))
			rows = max(rows, y+1)
		}
	}
	return trimmed{s, max(cols, minCols), max(rows, minRows)}
}

type trimmed struct {
	Screen
	cols, rows int
}

func (t trimmed) Width() int  { return min(t.cols, t.Screen.Width()) }
func (t trimmed) Height() int { return min(t.rows, t.Screen.Height()) }
