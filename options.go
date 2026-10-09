package termoscope

import (
	"time"

	"github.com/dimmkirr/termoscope/gif"
	"github.com/dimmkirr/termoscope/svg"
)

// Options tunes what RecordWith records and renders. The zero value is
// what Record uses. Fields that apply to both the SVG and the GIF are set
// once here so the two recordings always agree.
type Options struct {
	Interval   time.Duration // sampling interval, default 40ms
	Hold       time.Duration // how long the last frame stays before looping, default 2s
	MinCols    int           // minimum canvas width in cells; the canvas still grows to fit content
	MinRows    int           // minimum canvas height in cells; the canvas still grows to fit content
	EmbedEmoji bool          // embed the monochrome Noto Emoji face in the SVG (about 750 KB) for a render that is identical everywhere
}

func (o Options) interval() time.Duration {
	if o.Interval <= 0 {
		return 40 * time.Millisecond
	}
	return o.Interval
}

func (o Options) svg() svg.Options {
	return svg.Options{Hold: o.Hold, MinCols: o.MinCols, MinRows: o.MinRows, EmbedEmoji: o.EmbedEmoji}
}

func (o Options) gif() gif.Options {
	return gif.Options{Hold: o.Hold, MinCols: o.MinCols, MinRows: o.MinRows}
}
