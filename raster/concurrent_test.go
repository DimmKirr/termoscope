package raster

import (
	"sync"
	"testing"

	"github.com/charmbracelet/x/vt"
)

// TestRender_IsSafeForConcurrentUse mirrors parallel e2e sub-tests that
// call SavePNG at the same time. x/image font faces are not safe for
// concurrent use, so Render must serialize glyph drawing.
func TestRender_IsSafeForConcurrentUse(t *testing.T) {
	const workers, rounds = 16, 20
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			emu := vt.NewEmulator(40, 4)
			_, _ = emu.WriteString("Init  \x1b[38;2;57;211;83m▄ ▄ ▄\x1b[0m Processing Messaging Service 🚀\r\nrow two with more text to draw")
			for i := 0; i < rounds; i++ {
				if _, err := Render(emu); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
}
