package termproof

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dimmkirr/termproof/raster"
	"github.com/dimmkirr/termproof/svganim"
)

var (
	rootMu   sync.Mutex
	rootOver string
	runID    string
)

// SetResultsRoot overrides the directory that holds test/results. Empty
// restores auto-detection: walk up from the working directory to go.mod.
func SetResultsRoot(dir string) {
	rootMu.Lock()
	defer rootMu.Unlock()
	rootOver = dir
}

func resultsRoot() (string, error) {
	rootMu.Lock()
	over := rootOver
	rootMu.Unlock()
	if over != "" {
		return over, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

func sanitize(name string) string {
	return strings.NewReplacer("/", "_", " ", "_", string(filepath.Separator), "_").Replace(name)
}

// Dir returns test/results/<dateTimeISO>-<testName>, creating it. The
// timestamp is fixed for the whole test binary run.
func Dir(t *testing.T) string {
	t.Helper()
	rootMu.Lock()
	if runID == "" {
		runID = time.Now().UTC().Format("2006-01-02T15-04-05Z")
	}
	id := runID
	rootMu.Unlock()

	r, err := resultsRoot()
	if err != nil {
		t.Fatalf("termproof: locate module root: %v", err)
	}
	dir := filepath.Join(r, "test", "results", id+"-"+sanitize(t.Name()))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("termproof: mkdir %s: %v", dir, err)
	}
	return dir
}

// SavePNG renders the screen to <Dir>/<name>.png and returns the path.
func SavePNG(t *testing.T, s raster.Screen, name string) string {
	t.Helper()
	img, err := raster.Render(s)
	if err != nil {
		t.Fatalf("termproof: render: %v", err)
	}
	path := filepath.Join(Dir(t), name+".png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("termproof: create %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("termproof: encode: %v", err)
	}
	t.Logf("screenshot: %s", path)
	return path
}

// SaveSVG writes a static SVG of the screen to <Dir>/<name>.svg and returns
// the path.
func SaveSVG(t *testing.T, s svganim.Screen, name string) string {
	t.Helper()
	path := filepath.Join(Dir(t), name+".svg")
	writeFile(t, path, svganim.RenderStatic(svganim.Snapshot(s, 0), svganim.Options{}))
	return path
}

// Record samples the terminal until it exits and writes <Dir>/recording.svg
// when the test ends. Call it right after Start. The terminal is closed
// during cleanup, so a still-running program is killed and the recording
// ends there.
func Record(t *testing.T, tm *Terminal) {
	t.Helper()
	RecordWith(t, tm, 40*time.Millisecond, svganim.Options{})
}

// RecordWith is Record with an explicit sampling interval and SVG options.
func RecordWith(t *testing.T, tm *Terminal, interval time.Duration, opts svganim.Options) {
	t.Helper()
	frames := make(chan []svganim.Frame, 1)
	go func() { frames <- svganim.Record(tm, interval) }()
	t.Cleanup(func() {
		tm.Close()
		select {
		case fr := <-frames:
			writeFile(t, filepath.Join(Dir(t), "recording.svg"), svganim.Render(fr, opts))
		case <-time.After(5 * time.Second):
			t.Error("termproof: recorder did not finish")
		}
	})
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("termproof: write %s: %v", path, err)
	}
	t.Logf("artifact: %s", path)
}
