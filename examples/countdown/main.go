// Command countdown is a tiny in-place updating TUI used by termproof's own
// tests and README. It redraws a two-line screen each step without raw
// mode or the alternate screen, then exits 0.
//
//	Countdown  ▄ ▄ ▄
//	3
package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

const (
	grey  = "\x1b[38;2;33;38;45m"
	blue  = "\x1b[38;2;88;166;255m"
	green = "\x1b[38;2;57;211;83m"
	reset = "\x1b[0m"
)

func main() {
	steps := flag.Int("steps", 3, "number of steps to count down")
	pace := flag.Duration("pace", 300*time.Millisecond, "delay between steps")
	flag.Parse()

	p := func(format string, a ...any) { _, _ = fmt.Fprintf(os.Stdout, format, a...) }
	for i := 0; i <= *steps; i++ {
		if i > 0 {
			p("\r\x1b[1A\x1b[J") // back to the top line, erase below
		}
		p("Countdown  %s\n", tiles(i, *steps))
		if i < *steps {
			p("%s%d%s", blue, *steps-i, reset)
		} else {
			p("%sLiftoff%s", green, reset)
		}
		time.Sleep(*pace)
	}
	p("\n")
}

// tiles draws one square per step: done steps green, the current one blue,
// the rest grey.
func tiles(done, total int) string {
	s := ""
	for i := 0; i < total; i++ {
		switch {
		case i < done:
			s += green + "▄" + reset
		case i == done:
			s += blue + "▄" + reset
		default:
			s += grey + "▄" + reset
		}
		if i+1 < total {
			s += " "
		}
	}
	return s
}
