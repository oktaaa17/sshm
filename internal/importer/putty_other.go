//go:build !windows

package importer

import (
	"errors"

	"github.com/oktaaa17/sshm/internal/config"
)

func ImportPuTTYSessions(_ string) ([]config.Host, error) {
	return nil, errors.New("PuTTY registry import is only supported on Windows")
}
