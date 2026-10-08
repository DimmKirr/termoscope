// Package raster rasterizes a headless terminal screen into a hi-DPI RGBA
// image (JetBrains Mono at 2x, Noto Emoji for emoji). It is pure: no files,
// no testing imports. Use termproof.SavePNG to write a screenshot from a test.
package raster

import (
	"image"
	"image/color"
	"image/draw"
	"strings"
	"sync"
	"unicode/utf8"

	uv "github.com/charmbracelet/ultraviolet"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"github.com/dimmkirr/termproof/internal/fonts"
)

// Screen is the subset of a vt emulator the rasterizer needs.
type Screen interface {
	Width() int
	Height() int
	CellAt(x, y int) *uv.Cell
}

// Cell geometry in pixels. FontSize is the 2x (hi-DPI) size of a 14 px
// terminal font; width is the JetBrains Mono advance (0.6 em) and height a
// 1.25 em line, the 1:2 cell of a typical terminal. EmojiFontSize is chosen
// so a Noto Emoji glyph fits inside the two-cell span the emulator gives it.
const (
	FontSize      = 28
	CellWidth     = 17
	CellHeight    = 35
	EmojiFontSize = 26
)

var (
	defaultBg = color.RGBA{0x0d, 0x11, 0x17, 0xff}
	defaultFg = color.RGBA{0xc9, 0xd1, 0xd9, 0xff}

	faceOnce  sync.Once
	textFont  *sfnt.Font
	emojiFont *sfnt.Font
	face      font.Face
	emojiFace font.Face
	faceErr   error
)

func loadFaces() error {
	faceOnce.Do(func() {
		textFont, faceErr = opentype.Parse(fonts.JetBrainsMonoTTF)
		if faceErr != nil {
			return
		}
		face, faceErr = opentype.NewFace(textFont, &opentype.FaceOptions{Size: FontSize, DPI: 72, Hinting: font.HintingFull})
		if faceErr != nil {
			return
		}
		emojiFont, faceErr = opentype.Parse(fonts.NotoEmojiTTF)
		if faceErr != nil {
			return
		}
		emojiFace, faceErr = opentype.NewFace(emojiFont, &opentype.FaceOptions{Size: EmojiFontSize, DPI: 72, Hinting: font.HintingNone})
	})
	return faceErr
}

// hasGlyph reports whether f maps r to a real glyph (not .notdef).
func hasGlyph(f *sfnt.Font, buf *sfnt.Buffer, r rune) bool {
	idx, err := f.GlyphIndex(buf, r)
	return err == nil && idx != 0
}

// Render draws the screen into an RGBA image. Text is drawn with JetBrains
// Mono; a cell whose first rune JetBrains Mono lacks but Noto Emoji has is
// drawn with Noto Emoji, centered in the cell span the emulator assigned.
// Multi-rune clusters (variation selectors, skin tones, ZWJ sequences) are
// drawn as their first rune, since the rasterizer does no shaping.
func Render(s Screen) (*image.RGBA, error) {
	if err := loadFaces(); err != nil {
		return nil, err
	}
	w, h := s.Width(), s.Height()
	img := image.NewRGBA(image.Rect(0, 0, w*CellWidth, h*CellHeight))
	draw.Draw(img, img.Bounds(), image.NewUniform(defaultBg), image.Point{}, draw.Src)
	ascent := face.Metrics().Ascent.Ceil()
	d := &font.Drawer{Dst: img, Face: face}
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
			if !hasGlyph(textFont, &buf, r) && hasGlyph(emojiFont, &buf, r) {
				drawEmoji(d, r, px, py, span)
				continue
			}
			d.Face = face
			d.Dot = fixed.P(px, py+ascent)
			d.DrawString(c.Content)
		}
	}
	return img, nil
}

// drawEmoji draws r with the emoji face, centered both ways in a span px
// wide starting at (px, py).
func drawEmoji(d *font.Drawer, r rune, px, py, span int) {
	d.Face = emojiFace
	bounds, adv, _ := emojiFace.GlyphBounds(r)
	left := fixed.I(px) + (fixed.I(span)-adv)/2
	// bounds are relative to the dot with y growing downward, so the glyph's
	// vertical center sits at (Min.Y+Max.Y)/2 below the baseline.
	baseline := fixed.I(py) + fixed.I(CellHeight)/2 - (bounds.Min.Y+bounds.Max.Y)/2
	d.Dot = fixed.Point26_6{X: left, Y: baseline}
	d.DrawString(string(r))
}
