//go:build !windows

package credstore

func newWinCredStore(configDir string, passphraseFn func() ([]byte, error)) (Store, error) {
	return NewVaultStore(configDir, passphraseFn), nil
}

func newDPAPIStore(configDir string, passphraseFn func() ([]byte, error)) (Store, error) {
	return NewVaultStore(configDir, passphraseFn), nil
}
