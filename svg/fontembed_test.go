package svg

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"
)

// browser returns a headless Chromium/Chrome binary if one is installed.
// Browsers are the only common SVG viewers that honour @font-face with a
// data URI (librsvg ignores it), and GitHub's README rendering is a browser.
func browser() string {
	for _, name := range []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable", "chrome"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

// screenshot renders an SVG file with headless Chromium at the SVG's own size.
func screenshot(t *testing.T, bin, svgPath, outPath string, w, h int) image.Image {
	t.Helper()
	cmd := exec.Command(bin,
		"--headless=new", "--disable-gpu", "--no-sandbox", "--hide-scrollbars",
		"--force-device-scale-factor=2",
		"--window-size="+strconv.Itoa(w)+","+strconv.Itoa(h),
		"--screenshot="+outPath,
		"file://"+svgPath,
	)
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", bin, err, out)
	}
	f, err := os.Open(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// TestEmbeddedFont_IsUsedByBrowser proves the embedded JetBrains Mono is
// what a browser draws: the same frame rendered with and without the
// @font-face block must differ, and the difference must be in the text,
// not the tiles.
func TestEmbeddedFont_IsUsedByBrowser(t *testing.T) {
	bin := browser()
	if bin == "" {
		t.Skip("no headless Chromium/Chrome on PATH")
	}
	if testing.Short() {
		t.Skip("browser render is slow")
	}

	emu := vt.NewEmulator(40, 3)
	_, _ = emu.WriteString("Init  " + green("▄ ▄ ▄") + "\r\n─────────\r\n" + green("▄") + " Processing Messaging Service gy")
	frame := Snapshot(emu, 0)

	withFont := RenderStatic(frame, Options{})
	withoutFont := RenderStatic(frame, Options{NoEmbed: true})
	if !bytes.Contains(withFont, []byte("@font-face")) || bytes.Contains(withoutFont, []byte("@font-face")) {
		t.Fatal("test setup: embed flag not reflected in output")
	}
	w, h := svgSize(t, withFont)

	dir := t.TempDir()
	embedSVG := filepath.Join(dir, "embed.svg")
	plainSVG := filepath.Join(dir, "plain.svg")
	if err := os.WriteFile(embedSVG, withFont, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plainSVG, withoutFont, 0o644); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(90 * time.Second)
	a := screenshot(t, bin, embedSVG, filepath.Join(dir, "embed.png"), w, h)
	b := screenshot(t, bin, plainSVG, filepath.Join(dir, "plain.png"), w, h)
	if time.Now().After(deadline) {
		t.Log("warning: browser renders took unusually long")
	}

	textDiff, tileDiff := regionDiff(a, b, frame, Options{}.withDefaults())
	t.Logf("differing pixels: text region %d, tile region %d", textDiff, tileDiff)
	if textDiff == 0 {
		t.Fatal("text renders identically with and without the embedded font: the browser is not using it")
	}
	if tileDiff != 0 {
		t.Fatalf("tiles must not depend on the font, got %d differing pixels", tileDiff)
	}
	// Rendering twice with the font must be stable (guards against a flaky
	// comparison masquerading as a font difference).
	c := screenshot(t, bin, embedSVG, filepath.Join(dir, "embed2.png"), w, h)
	if again, _ := regionDiff(a, c, frame, Options{}.withDefaults()); again != 0 {
		t.Fatalf("same SVG rendered differently twice (%d pixels); comparison is not reliable", again)
	}
}

// regionDiff counts differing pixels inside text cells and inside tile
// cells separately. Images are 2x the SVG size.
func regionDiff(a, b image.Image, f Frame, o Options) (text, tiles int) {
	cw, ch := o.cell()
	const scale = 2
	for y, row := range f.Lines {
		for x, c := range row {
			if c.Content == " " {
				continue
			}
			x0 := int((o.Padding + float64(x)*cw) * scale)
			y0 := int((o.Padding + float64(y)*ch) * scale)
			x1 := int((o.Padding + float64(x+1)*cw) * scale)
			y1 := int((o.Padding + float64(y+1)*ch) * scale)
			n := 0
			for py := y0; py < y1 && py < a.Bounds().Max.Y; py++ {
				for px := x0; px < x1 && px < a.Bounds().Max.X; px++ {
					r1, g1, b1, _ := a.At(px, py).RGBA()
					r2, g2, b2, _ := b.At(px, py).RGBA()
					if r1 != r2 || g1 != g2 || b1 != b2 {
						n++
					}
				}
			}
			if isBlock(c.Content) {
				tiles += n
			} else {
				text += n
			}
		}
	}
	return text, tiles
}

func svgSize(t *testing.T, svg []byte) (w, h int) {
	t.Helper()
	s := string(svg)
	get := func(attr string) int {
		i := strings.Index(s, attr+`="`)
		if i < 0 {
			t.Fatalf("missing %s", attr)
		}
		i += len(attr) + 2
		j := strings.Index(s[i:], `"`)
		v := 0
		for _, r := range s[i : i+j] {
			if r == '.' {
				break
			}
			v = v*10 + int(r-'0')
		}
		return v + 1
	}
	return get("width"), get("height")
}

// TestEmbeddedEmojiFont_IsUsedByBrowser proves Options.EmbedEmoji makes a
// browser draw emoji from the bundled Noto Emoji: the emoji cells must
// render differently with and without the embedded face. Without it the
// viewer falls back to whatever emoji font it has, if any.
func TestEmbeddedEmojiFont_IsUsedByBrowser(t *testing.T) {
	bin := browser()
	if bin == "" {
		t.Skip("no headless Chromium/Chrome on PATH")
	}
	if testing.Short() {
		t.Skip("browser render is slow")
	}

	emu := vt.NewEmulator(20, 1)
	_, _ = emu.WriteString("Ship 🚀 ✅ ok")
	frame := Snapshot(emu, 0)

	with := RenderStatic(frame, Options{EmbedEmoji: true})
	without := RenderStatic(frame, Options{})
	w, h := svgSize(t, with)

	dir := t.TempDir()
	withSVG := filepath.Join(dir, "emoji.svg")
	withoutSVG := filepath.Join(dir, "plain.svg")
	if err := os.WriteFile(withSVG, with, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(withoutSVG, without, 0o644); err != nil {
		t.Fatal(err)
	}
	a := screenshot(t, bin, withSVG, filepath.Join(dir, "emoji.png"), w, h)
	b := screenshot(t, bin, withoutSVG, filepath.Join(dir, "plain.png"), w, h)

	emojiDiff, textDiff := wideDiff(a, b, frame, Options{}.withDefaults())
	t.Logf("differing pixels: emoji cells %d, text cells %d", emojiDiff, textDiff)
	if emojiDiff == 0 {
		t.Fatal("emoji render identically with and without the embedded Noto Emoji: the browser is not using it")
	}
	if textDiff != 0 {
		t.Fatalf("plain text must not depend on the emoji font, got %d differing pixels", textDiff)
	}
}

// wideDiff counts differing pixels inside wide (emoji) cells and inside
// single-width text cells separately, walking columns by cell width.
func wideDiff(a, b image.Image, f Frame, o Options) (wide, text int) {
	cw, ch := o.cell()
	const scale = 2
	for y, row := range f.Lines {
		col := 0
		for _, c := range row {
			w := c.width()
			if c.Content != " " {
				x0 := int((o.Padding + float64(col)*cw) * scale)
				y0 := int((o.Padding + float64(y)*ch) * scale)
				x1 := int((o.Padding + float64(col+w)*cw) * scale)
				y1 := int((o.Padding + float64(y+1)*ch) * scale)
				n := 0
				for py := y0; py < y1 && py < a.Bounds().Max.Y; py++ {
					for px := x0; px < x1 && px < a.Bounds().Max.X; px++ {
						r1, g1, b1, _ := a.At(px, py).RGBA()
						r2, g2, b2, _ := b.At(px, py).RGBA()
						if r1 != r2 || g1 != g2 || b1 != b2 {
							n++
						}
					}
				}
				if w > 1 {
					wide += n
				} else {
					text += n
				}
			}
			col += w
		}
	}
	return wide, text
}
