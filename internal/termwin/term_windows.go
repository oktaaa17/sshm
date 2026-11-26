//go:build windows

package termwin

import (
	"context"
	"os"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/term"
)

// SessionState holds original Windows console modes so they can be cleanly restored.
type SessionState struct {
	stdinHandle  windows.Handle
	stdoutHandle windows.Handle
	inMode       uint32
	outMode      uint32
	hasInMode    bool
	hasOutMode   bool
	rawState     *term.State
	stdinFD      int
}

// EnterRawVT puts Windows stdin/stdout into raw + Virtual Terminal Processing mode.
func EnterRawVT() (*SessionState, error) {
	st := &SessionState{
		stdinHandle:  windows.Handle(os.Stdin.Fd()),
		stdoutHandle: windows.Handle(os.Stdout.Fd()),
		stdinFD:      int(os.Stdin.Fd()),
	}

	if err := windows.GetConsoleMode(st.stdoutHandle, &st.outMode); err == nil {
		st.hasOutMode = true
		newOut := st.outMode | windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING | windows.DISABLE_NEWLINE_AUTO_RETURN
		_ = windows.SetConsoleMode(st.stdoutHandle, newOut)
	}

	if err := windows.GetConsoleMode(st.stdinHandle, &st.inMode); err == nil {
		st.hasInMode = true
		newIn := (st.inMode &^ (windows.ENABLE_ECHO_INPUT | windows.ENABLE_LINE_INPUT | windows.ENABLE_PROCESSED_INPUT)) | windows.ENABLE_VIRTUAL_TERMINAL_INPUT
		_ = windows.SetConsoleMode(st.stdinHandle, newIn)
	} else if term.IsTerminal(st.stdinFD) {
		raw, err := term.MakeRaw(st.stdinFD)
		if err != nil {
			return st, err
		}
		st.rawState = raw
	}

	return st, nil
}

// Restore restores the original console modes. Safe to call multiple times.
func (s *SessionState) Restore() {
	if s == nil {
		return
	}
	if s.rawState != nil {
		_ = term.Restore(s.stdinFD, s.rawState)
		s.rawState = nil
	}
	if s.hasInMode {
		_ = windows.SetConsoleMode(s.stdinHandle, s.inMode)
		s.hasInMode = false
	}
	if s.hasOutMode {
		_ = windows.SetConsoleMode(s.stdoutHandle, s.outMode)
		s.hasOutMode = false
	}
}

// GetSize returns the current terminal width and height.
func GetSize() (int, int) {
	if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 && h > 0 {
		return w, h
	}
	if w, h, err := term.GetSize(int(os.Stdin.Fd())); err == nil && w > 0 && h > 0 {
		return w, h
	}
	return 80, 24
}

// WatchResize polls Windows console buffer dimensions and invokes onResize when changed.
func WatchResize(ctx context.Context, onResize func(w, h int)) {
	lastW, lastH := GetSize()
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w, h := GetSize()
			if w != lastW || h != lastH {
				lastW, lastH = w, h
				onResize(w, h)
			}
		}
	}
}

// IsTerminal reports whether stdin is connected to an interactive terminal.
func IsTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// ReadPassword reads a password without echoing characters.
func ReadPassword() ([]byte, error) {
	return term.ReadPassword(int(os.Stdin.Fd()))
}
