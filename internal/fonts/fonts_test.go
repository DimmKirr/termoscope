package fonts

import (
	"bytes"
	"testing"

	"golang.org/x/image/font/opentype"
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
