// Package raster rasterizes a headless terminal screen into a hi-DPI RGBA
// image (JetBrains Mono at 2x). It is pure: no files, no testing imports.
// Use termproof.SavePNG to write a screenshot from a test.
package raster

import (
	"image"
	"image/color"
	"image/draw"
	"strings"
	"sync"

	uv "github.com/charmbracelet/ultraviolet"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
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
// 1.25 em line, the 1:2 cell of a typical terminal.
const (
	FontSize   = 28
	CellWidth  = 17
	CellHeight = 35
)

var (
	defaultBg = color.RGBA{0x0d, 0x11, 0x17, 0xff}
	defaultFg = color.RGBA{0xc9, 0xd1, 0xd9, 0xff}

	faceOnce sync.Once
	face     font.Face
	faceErr  error
)

func loadFace() (font.Face, error) {
	faceOnce.Do(func() {
		var f *opentype.Font
		f, faceErr = opentype.Parse(fonts.JetBrainsMonoTTF)
		if faceErr != nil {
			return
		}
		face, faceErr = opentype.NewFace(f, &opentype.FaceOptions{Size: FontSize, DPI: 72, Hinting: font.HintingFull})
	})
	return face, faceErr
}

// Render draws the screen into an RGBA image.
func Render(s Screen) (*image.RGBA, error) {
	fc, err := loadFace()
	if err != nil {
		return nil, err
	}
	w, h := s.Width(), s.Height()
	img := image.NewRGBA(image.Rect(0, 0, w*CellWidth, h*CellHeight))
	draw.Draw(img, img.Bounds(), image.NewUniform(defaultBg), image.Point{}, draw.Src)
	ascent := fc.Metrics().Ascent.Ceil()
	d := &font.Drawer{Dst: img, Face: fc}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := s.CellAt(x, y)
			if c == nil {
				continue
			}
			px, py := x*CellWidth, y*CellHeight
			if c.Style.Bg != nil {
				draw.Draw(img, image.Rect(px, py, px+CellWidth*max(c.Width, 1), py+CellHeight), image.NewUniform(c.Style.Bg), image.Point{}, draw.Src)
			}
			if strings.TrimSpace(c.Content) == "" {
				continue
			}
			fg := color.Color(defaultFg)
			if c.Style.Fg != nil {
				fg = c.Style.Fg
			}
			d.Src = image.NewUniform(fg)
			d.Dot = fixed.P(px, py+ascent)
			d.DrawString(c.Content)
		}
	}
	return img, nil
}
