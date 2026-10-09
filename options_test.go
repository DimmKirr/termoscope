package termoscope

import (
	"testing"
	"time"
)

func TestOptions_MapsToSVGAndGIF(t *testing.T) {
	o := Options{Hold: 3 * time.Second, MinCols: 20, MinRows: 4, EmbedEmoji: true}
	s := o.svg()
	if s.Hold != 3*time.Second || s.MinCols != 20 || s.MinRows != 4 || !s.EmbedEmoji {
		t.Errorf("svg options not mapped: %+v", s)
	}
	g := o.gif()
	if g.Hold != 3*time.Second || g.MinCols != 20 || g.MinRows != 4 {
		t.Errorf("gif options not mapped: %+v", g)
	}
}

func TestOptions_IntervalDefaultsTo40ms(t *testing.T) {
	if got := (Options{}).interval(); got != 40*time.Millisecond {
		t.Errorf("default interval = %v, want 40ms", got)
	}
	if got := (Options{Interval: time.Second}).interval(); got != time.Second {
		t.Errorf("interval = %v, want 1s", got)
	}
}
