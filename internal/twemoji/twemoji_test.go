package twemoji

import "testing"

func TestKey(t *testing.T) {
	for _, tc := range []struct {
		in       string
		keepFE0F bool
		want     string
	}{
		{"🚀", false, "1f680"},
		{"❤️", false, "2764"},
		{"❤️", true, "2764-fe0f"},
		{"👍🏽", false, "1f44d-1f3fd"},
		{"👨‍👩‍👧", false, "1f468-200d-1f469-200d-1f467"},
		{"🇨🇿", false, "1f1e8-1f1ff"},
	} {
		if got := key(tc.in, tc.keepFE0F); got != tc.want {
			t.Errorf("key(%q, %v) = %q, want %q", tc.in, tc.keepFE0F, got, tc.want)
		}
	}
}

func TestLookup_ResolvesSequencesAndFallsBack(t *testing.T) {
	for _, c := range []string{"🚀", "✅", "❤️", "👍🏽", "👨‍👩‍👧", "🇨🇿", "⏳", "🎉"} {
		img, ok := Lookup(c)
		if !ok || img == nil {
			t.Errorf("no image for %q", c)
			continue
		}
		if b := img.Bounds(); b.Dx() != 72 || b.Dy() != 72 {
			t.Errorf("%q: bounds %v, want 72x72", c, b)
		}
	}
	// Not emoji at all.
	if _, ok := Lookup("A"); ok {
		t.Error("letter A must not resolve to an emoji")
	}
	if _, ok := Lookup("▄"); ok {
		t.Error("block glyph must not resolve to an emoji")
	}
	// Cached negative and positive results are stable.
	if _, ok := Lookup("A"); ok {
		t.Error("cached negative lookup changed")
	}
	if _, ok := Lookup("🚀"); !ok {
		t.Error("cached positive lookup changed")
	}
}
