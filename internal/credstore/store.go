package credstore

import (
	"errors"
	"fmt"
	"strings"
)

// ErrNotFound is returned when no credential exists for a given reference.
var ErrNotFound = errors.New("credential not found")

// Store persists secrets securely outside plaintext config files.
type Store interface {
	Set(ref string, secret []byte) error
	Get(ref string) ([]byte, error)
	Delete(ref string) error
	BackendName() string
}

// ZeroBytes overwrites sensitive byte slices in memory.
func ZeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// KeyPassphraseRef returns the credential reference for a host's SSH key passphrase.
func KeyPassphraseRef(hostID string) string {
	return "sshm:key:" + hostID
}

// PasswordRef returns the credential reference for a host's SSH password.
func PasswordRef(hostID string) string {
	return "sshm:" + hostID
}

// New constructs a Store for the requested backend ("wincred", "dpapi", "vault").
func New(backend, configDir string, passphraseFn func() ([]byte, error)) (Store, error) {
	switch strings.ToLower(strings.TrimSpace(backend)) {
	case "", "wincred":
		return newWinCredStore(configDir, passphraseFn)
	case "dpapi":
		return newDPAPIStore(configDir, passphraseFn)
	case "vault":
		return NewVaultStore(configDir, passphraseFn), nil
	default:
		return nil, fmt.Errorf("unsupported credential backend %q (expected wincred, dpapi, or vault)", backend)
	}
}
