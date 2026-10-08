package fonts

import (
	"bytes"
	"testing"

	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
)

func TestTTF_ParsesAndMatchesMetrics(t *testing.T) {
	f, err := opentype.Parse(JetBrainsMonoTTF)
	if err != nil {
		t.Fatal(err)
	}
	if upem := f.UnitsPerEm(); upem != 1000 {
		t.Fatalf("unitsPerEm %d, want 1000", upem)
	}
}

func TestWOFF2_HasSignature(t *testing.T) {
	if !bytes.HasPrefix(JetBrainsMonoWOFF2, []byte("wOF2")) {
		t.Fatal("embedded woff2 missing wOF2 signature")
	}
	if len(JetBrainsMonoWOFF2) > 200_000 {
		t.Fatalf("woff2 unexpectedly large: %d bytes", len(JetBrainsMonoWOFF2))
	}
}

func TestNotoEmoji_ParsesAndCoversEmoji(t *testing.T) {
	f, err := opentype.Parse(NotoEmojiTTF)
	if err != nil {
		t.Fatal(err)
	}
	var buf sfnt.Buffer
	for _, r := range []rune{'🚀', '✅', '🎉', '⏳'} {
		idx, err := f.GlyphIndex(&buf, r)
		if err != nil || idx == 0 {
			t.Errorf("Noto Emoji lacks %q (%U): idx=%d err=%v", r, r, idx, err)
		}
	}
	if !bytes.HasPrefix(NotoEmojiWOFF, []byte("wOFF")) {
		t.Fatal("embedded Noto Emoji woff missing wOFF signature")
	}
}
