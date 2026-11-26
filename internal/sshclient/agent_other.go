//go:build !windows

package sshclient

import (
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh/agent"
)

func dialSSHAgent() (agent.ExtendedAgent, io.Closer, error) {
	sock := filepath.Clean(os.Getenv("SSH_AUTH_SOCK"))
	if sock == "" || sock == "." || !filepath.IsAbs(sock) {
		return nil, nil, errors.New("SSH_AUTH_SOCK is not set to an absolute socket path")
	}
	info, err := os.Stat(sock) // #nosec G703 -- standard POSIX SSH_AUTH_SOCK environment variable verified as absolute path
	if err != nil || info.Mode()&os.ModeSocket == 0 {
		return nil, nil, errors.New("SSH_AUTH_SOCK does not point to a valid Unix socket")
	}
	conn, err := net.Dial("unix", sock) // #nosec G704 -- validated local Unix domain socket from SSH_AUTH_SOCK
	if err != nil {
		return nil, nil, err
	}
	return agent.NewClient(conn), conn, nil
}
