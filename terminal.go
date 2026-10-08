// Package termproof runs a program under a headless pseudo-terminal so a
// test can read what a user would see, wait for screen states, and keep
// PNG and animated SVG evidence of every checked screen.
//
// A typical test builds its binary, calls [Start], registers [Record] so a
// recording.svg lands beside the screenshots, waits with [Terminal.WaitFor]
// or [Terminal.WaitUntil], and calls [SavePNG] and [SaveSVG] at each
// assertion point. Artifacts go to test/results/<dateTimeISO>-<testName>/
// under the module root of the test being run.
package termproof

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
	"github.com/charmbracelet/x/xpty"
)

// Terminal is a running child process with its screen.
type Terminal struct {
	pty    xpty.Pty
	emu    *vt.SafeEmulator
	cmd    *exec.Cmd
	mu     sync.Mutex    // serializes emulator writes and reads
	exited chan struct{} // closed once the process has exited
	err    error         // exit error, valid after exited is closed
	once   sync.Once
}

// Start launches name with args in a width×height PTY. The environment
// advertises a truecolor xterm so colors are emitted.
func Start(ctx context.Context, width, height int, name string, args ...string) (*Terminal, error) {
	pty, err := xpty.NewPty(width, height)
	if err != nil {
		return nil, fmt.Errorf("create pty: %w", err)
	}
	emu := vt.NewSafeEmulator(width, height)
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(cmd.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor", "CLICOLOR_FORCE=1")
	if err := pty.Start(cmd); err != nil {
		_ = pty.Close()
		return nil, fmt.Errorf("start %s: %w", name, err)
	}
	t := &Terminal{pty: pty, emu: emu, cmd: cmd, exited: make(chan struct{})}
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := pty.Read(buf)
			if n > 0 {
				t.mu.Lock()
				_, _ = emu.Write(buf[:n])
				t.mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()
	go func() {
		t.err = xpty.WaitProcess(ctx, cmd)
		close(t.exited)
	}()
	return t, nil
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

// StripANSI removes CSI escape sequences from s.
func StripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

// Screen returns the visible text without escape codes.
func (t *Terminal) Screen() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return StripANSI(t.emu.Render())
}

// Width reports the emulator width in cells.
func (t *Terminal) Width() int { return t.emu.Width() }

// Height reports the emulator height in cells.
func (t *Terminal) Height() int { return t.emu.Height() }

// CellAt returns a copy of one screen cell, or nil outside the screen. A
// copy is returned because the emulator keeps writing while tests read.
func (t *Terminal) CellAt(x, y int) *uv.Cell {
	t.mu.Lock()
	defer t.mu.Unlock()
	c := t.emu.CellAt(x, y)
	if c == nil {
		return nil
	}
	cp := *c
	return &cp
}

// Line returns the text of screen row y, right-trimmed.
func (t *Terminal) Line(y int) string {
	var b strings.Builder
	for x := 0; x < t.Width(); x++ {
		if c := t.CellAt(x, y); c != nil && c.Width > 0 {
			b.WriteString(c.Content)
		}
	}
	return strings.TrimRight(b.String(), " ")
}

// Send writes raw bytes to the child's stdin.
func (t *Terminal) Send(s string) error {
	_, err := t.pty.Write([]byte(s))
	return err
}

// SendLine writes s followed by Enter.
func (t *Terminal) SendLine(s string) error { return t.Send(s + "\r") }

// WaitFor polls the screen until it contains substr or ctx expires.
func (t *Terminal) WaitFor(ctx context.Context, substr string) (string, error) {
	err := t.WaitUntil(ctx, func(t *Terminal) bool { return strings.Contains(t.Screen(), substr) })
	if err != nil {
		return t.Screen(), fmt.Errorf("waiting for %q: %w", substr, err)
	}
	return t.Screen(), nil
}

// WaitUntil polls until pred reports true or ctx expires. The error on
// timeout includes the current screen.
func (t *Terminal) WaitUntil(ctx context.Context, pred func(*Terminal) bool) error {
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		if pred(t) {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for condition; screen:\n%s", t.Screen())
		case <-tick.C:
		}
	}
}

// Done is closed when the process has exited. Safe for multiple waiters.
func (t *Terminal) Done() <-chan struct{} { return t.exited }

// Wait blocks until the process exits and returns its error. It may be
// called more than once.
func (t *Terminal) Wait() error {
	<-t.exited
	return t.err
}

// Close releases the PTY and kills the process if still running.
func (t *Terminal) Close() {
	t.once.Do(func() {
		if t.cmd.Process != nil {
			_ = t.cmd.Process.Kill()
		}
		_ = t.pty.Close()
	})
}
