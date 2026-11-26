//go:build !windows

package sshclient

import (
	"errors"
	"io"
	"net"
	"os"

	"golang.org/x/crypto/ssh/agent"
)

func dialSSHAgent() (agent.ExtendedAgent, io.Closer, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, nil, errors.New("SSH_AUTH_SOCK is not set")
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, nil, err
	}
	return agent.NewClient(conn), conn, nil
}
