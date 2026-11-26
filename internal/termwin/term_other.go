//go:build !windows

package termwin

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"
)

type SessionState struct {
	rawState *term.State
	stdinFD  int
}

func EnterRawVT() (*SessionState, error) {
	fd := int(os.Stdin.Fd())
	st := &SessionState{stdinFD: fd}
	if term.IsTerminal(fd) {
		raw, err := term.MakeRaw(fd)
		if err != nil {
			return nil, err
		}
		st.rawState = raw
	}
	return st, nil
}

func (s *SessionState) Restore() {
	if s != nil && s.rawState != nil {
		_ = term.Restore(s.stdinFD, s.rawState)
		s.rawState = nil
	}
}

func GetSize() (int, int) {
	if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 && h > 0 {
		return w, h
	}
	if w, h, err := term.GetSize(int(os.Stdin.Fd())); err == nil && w > 0 && h > 0 {
		return w, h
	}
	return 80, 24
}

func WatchResize(ctx context.Context, onResize func(w, h int)) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	defer signal.Stop(ch)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ch:
			w, h := GetSize()
			onResize(w, h)
		}
	}
}

func IsTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func ReadPassword() ([]byte, error) {
	return term.ReadPassword(int(os.Stdin.Fd()))
}
