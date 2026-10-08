package main

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_Usage(t *testing.T) {
	var buf bytes.Buffer
	if code := run(nil, &buf); code != 2 || !strings.Contains(buf.String(), "usage:") {
		t.Fatalf("no args: code %d, out %q", code, buf.String())
	}
	buf.Reset()
	if code := run([]string{"bogus"}, &buf); code != 2 || !strings.Contains(buf.String(), "unknown command") {
		t.Fatalf("unknown command: code %d, out %q", code, buf.String())
	}
	buf.Reset()
	if code := run([]string{"record", "-o", "x.svg"}, &buf); code != 2 || !strings.Contains(buf.String(), "missing command") {
		t.Fatalf("missing command: code %d, out %q", code, buf.String())
	}
}

func TestRecord_WritesSVGAndPNGAndPropagatesExitCode(t *testing.T) {
	dir := t.TempDir()
	svg := filepath.Join(dir, "out.svg")
	pngPath := filepath.Join(dir, "last.png")
	var buf bytes.Buffer
	code := run([]string{"record", "-o", svg, "-png", pngPath, "-cols", "20", "-rows", "3", "-sample", "5ms", "--",
		"sh", "-c", `printf 'a'; sleep 0.05; printf '\033[38;2;57;211;83m\342\226\204\033[0m'; sleep 0.05; exit 3`}, &buf)
	if code != 3 {
		t.Fatalf("exit code %d, want child's 3; stderr:\n%s", code, buf.String())
	}
	data, err := os.ReadFile(svg)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, `fill="#39d353"`) || !strings.Contains(s, "@keyframes") || strings.Count(s, `class="f"`) < 2 {
		t.Fatalf("svg must be animated with the green tile:\n%s", s)
	}
	f, err := os.Open(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if _, err := png.Decode(f); err != nil {
		t.Fatalf("png: %v", err)
	}
	if !strings.Contains(buf.String(), "wrote "+svg) {
		t.Fatalf("stderr should report the output: %q", buf.String())
	}
}
