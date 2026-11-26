//go:build windows

package sshclient

import (
	"io"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/crypto/ssh/agent"
)

const windowsOpenSSHAgentPipe = `\\.\pipe\openssh-ssh-agent`

func dialSSHAgent() (agent.ExtendedAgent, io.Closer, error) {
	timeout := 500 * time.Millisecond
	conn, err := winio.DialPipe(windowsOpenSSHAgentPipe, &timeout)
	if err != nil {
		return nil, nil, err
	}
	return agent.NewClient(conn), conn, nil
}
