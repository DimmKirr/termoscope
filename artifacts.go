package termoscope

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dimmkirr/termoscope/gif"
	"github.com/dimmkirr/termoscope/raster"
	"github.com/dimmkirr/termoscope/svg"
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
		t.Fatalf("termoscope: locate module root: %v", err)
	}
	dir := filepath.Join(r, "test", "results", id+"-"+sanitize(t.Name()))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("termoscope: mkdir %s: %v", dir, err)
	}
	return dir
}

// SavePNG renders the screen to <Dir>/<name>.png and returns the path. Like
// the SVG, the image is cropped to the cells holding content and padded by
// one cell height (raster.Margin) on every side, at 2x.
func SavePNG(t *testing.T, s raster.Screen, name string) string {
	t.Helper()
	img, err := raster.Render(raster.Trim(s, 0, 0))
	if err != nil {
		t.Fatalf("termoscope: render: %v", err)
	}
	img = raster.Pad([]*image.RGBA{img}, raster.Margin)[0]
	path := filepath.Join(Dir(t), name+".png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("termoscope: create %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("termoscope: encode: %v", err)
	}
	t.Logf("screenshot: %s", path)
	return path
}

// SaveSVG writes a static SVG of the screen to <Dir>/<name>.svg and returns
// the path.
func SaveSVG(t *testing.T, s svg.Screen, name string) string {
	t.Helper()
	path := filepath.Join(Dir(t), name+".svg")
	writeFile(t, path, svg.RenderStatic(svg.Snapshot(s, 0), svg.Options{}))
	return path
}

// Record samples the terminal until it exits and writes <Dir>/recording.svg
// and <Dir>/recording.gif when the test ends. Call it right after Start.
// The terminal is closed during cleanup, so a still-running program is
// killed and the recording ends there. The SVG stays crisp at any zoom and
// shows the viewer's color emoji; the GIF is pixel-exact with the bundled
// fonts and needs nothing from the viewer.
func Record(t *testing.T, tm *Terminal) {
	t.Helper()
	RecordWith(t, tm, Options{})
}

// RecordWith is Record with Options: sampling interval, hold, minimum
// canvas size and emoji embedding. The SVG and the GIF share them.
func RecordWith(t *testing.T, tm *Terminal, opts Options) {
	t.Helper()
	frames := make(chan []svg.Frame, 1)
	go func() { frames <- svg.Record(tm, opts.interval()) }()
	t.Cleanup(func() {
		tm.Close()
		select {
		case fr := <-frames:
			writeFile(t, filepath.Join(Dir(t), "recording.svg"), svg.Render(fr, opts.svg()))
			g, err := gif.Render(fr, opts.gif())
			if err != nil {
				t.Errorf("termoscope: render gif: %v", err)
				return
			}
			writeFile(t, filepath.Join(Dir(t), "recording.gif"), g)
		case <-time.After(5 * time.Second):
			t.Error("termoscope: recorder did not finish")
		}
	})
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("termoscope: write %s: %v", path, err)
	}
	t.Logf("artifact: %s", path)
}
