// Command emoji is a tiny in-place updating TUI used by termoscope's own
// tests and README to prove wide emoji cells render and align. It draws a
// release checklist whose steps flip from pending to done, then exits 0.
//
//	Release 🚀 v1.2
//	✅ build
//	✅ test
//	🎉 shipped
package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

const (
	grey  = "\x1b[38;2;139;148;158m"
	blue  = "\x1b[38;2;88;166;255m"
	green = "\x1b[38;2;57;211;83m"
	reset = "\x1b[0m"
)

var steps = []string{"build", "test", "ship"}

func main() {
	pace := flag.Duration("pace", 300*time.Millisecond, "delay between steps")
	flag.Parse()

	p := func(format string, a ...any) { _, _ = fmt.Fprintf(os.Stdout, format, a...) }
	for done := 0; done <= len(steps); done++ {
		if done > 0 {
			p("\r\x1b[%dA\x1b[J", len(steps)) // back to the top line, erase below
		}
		p("Release %s🚀%s v1.2\n", blue, reset)
		for i, s := range steps {
			switch {
			case done == len(steps) && i == len(steps)-1:
				p("%s🎉 shipped%s", green, reset)
			case i < done:
				p("%s✅ %s%s", green, s, reset)
			default:
				p("%s⏳ %s%s", grey, s, reset)
			}
			if i+1 < len(steps) {
				p("\n")
			}
		}
		time.Sleep(*pace)
	}
}
