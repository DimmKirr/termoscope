// Package twemoji embeds the Twemoji 72x72 PNG set (jdecked/twemoji, graphics
// CC-BY 4.0, see LICENSE-GRAPHICS) and resolves a grapheme cluster to its
// image. Lookup is by the whole sequence, so skin tones, flags and ZWJ
// families resolve to the right picture without any text shaping.
package twemoji

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/png"
	"strings"
	"sync"
)

//go:embed assets/*.png
var assets embed.FS

// Version is the upstream Twemoji release the assets come from.
const Version = "17.0.3"

var (
	mu    sync.Mutex
	cache = map[string]*image.RGBA{} // cluster → decoded image, nil when absent
)

// key returns the Twemoji file stem for a cluster: lowercase hex code points
// joined by "-". Twemoji strips U+FE0F from names unless the sequence also
// contains U+200D (ZWJ), so both forms are tried by Lookup.
func key(cluster string, keepFE0F bool) string {
	var b strings.Builder
	for i, r := range cluster {
		if r == 0xFE0F && !keepFE0F {
			continue
		}
		if i > 0 && b.Len() > 0 {
			b.WriteByte('-')
		}
		fmt.Fprintf(&b, "%x", r)
	}
	return b.String()
}

// Has reports whether cluster has a Twemoji image.
func Has(cluster string) bool {
	_, ok := Lookup(cluster)
	return ok
}

// Lookup returns the decoded 72x72 image for cluster, trying the exact
// sequence, then the sequence without U+FE0F, then the base code point
// alone. Results are cached; the returned image must not be modified.
func Lookup(cluster string) (*image.RGBA, bool) {
	mu.Lock()
	defer mu.Unlock()
	if img, seen := cache[cluster]; seen {
		return img, img != nil
	}
	img := decodeFirst(key(cluster, true), key(cluster, false), baseKey(cluster))
	cache[cluster] = img
	return img, img != nil
}

func baseKey(cluster string) string {
	for _, r := range cluster {
		return fmt.Sprintf("%x", r)
	}
	return ""
}

func decodeFirst(keys ...string) *image.RGBA {
	tried := map[string]bool{}
	for _, k := range keys {
		if k == "" || tried[k] {
			continue
		}
		tried[k] = true
		data, err := assets.ReadFile("assets/" + k + ".png")
		if err != nil {
			continue
		}
		src, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			continue
		}
		rgba := image.NewRGBA(src.Bounds())
		for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
			for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
				rgba.Set(x, y, src.At(x, y))
			}
		}
		return rgba
	}
	return nil
}
