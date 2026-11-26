package credstore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/argon2"
)

const (
	vaultMagic   = "SSHM1" // 5 bytes
	vaultSaltLen = 16
	vaultKeyLen  = 32
)

// EncryptBundle encrypts arbitrary bytes using Argon2id + AES-256-GCM.
func EncryptBundle(plaintext, passphrase []byte) ([]byte, error) {
	if len(passphrase) == 0 {
		return nil, errors.New("passphrase cannot be empty")
	}
	salt := make([]byte, vaultSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	key := argon2.IDKey(passphrase, salt, 3, 64*1024, 4, vaultKeyLen)
	defer ZeroBytes(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, []byte(vaultMagic))

	out := make([]byte, 0, len(vaultMagic)+len(salt)+len(nonce)+len(ciphertext))
	out = append(out, []byte(vaultMagic)...)
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)
	return out, nil
}

// DecryptBundle decrypts an EncryptBundle payload.
func DecryptBundle(blob, passphrase []byte) ([]byte, error) {
	if len(passphrase) == 0 {
		return nil, errors.New("passphrase cannot be empty")
	}
	minHeader := len(vaultMagic) + vaultSaltLen + 12
	if len(blob) < minHeader {
		return nil, errors.New("invalid encrypted bundle format")
	}
	if string(blob[:len(vaultMagic)]) != vaultMagic {
		return nil, errors.New("unsupported encrypted bundle version")
	}
	offset := len(vaultMagic)
	salt := blob[offset : offset+vaultSaltLen]
	offset += vaultSaltLen

	key := argon2.IDKey(passphrase, salt, 3, 64*1024, 4, vaultKeyLen)
	defer ZeroBytes(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(blob) < offset+nonceSize {
		return nil, errors.New("corrupted encrypted bundle")
	}
	nonce := blob[offset : offset+nonceSize]
	ciphertext := blob[offset+nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte(vaultMagic))
	if err != nil {
		return nil, errors.New("decryption failed (wrong passphrase or corrupted vault)")
	}
	return plaintext, nil
}

// VaultStore is a portable AES-256-GCM + Argon2id encrypted file store.
type VaultStore struct {
	mu           sync.Mutex
	path         string
	passphraseFn func() ([]byte, error)
	cachedPass   []byte
}

func NewVaultStore(configDir string, passphraseFn func() ([]byte, error)) *VaultStore {
	return &VaultStore{
		path:         filepath.Join(configDir, "vault.enc"),
		passphraseFn: passphraseFn,
	}
}

func (v *VaultStore) BackendName() string {
	return "Encrypted Vault (AES-256-GCM)"
}

// Close zeroes any cached vault passphrase in memory.
func (v *VaultStore) Close() {
	v.mu.Lock()
	defer v.mu.Unlock()
	if len(v.cachedPass) > 0 {
		ZeroBytes(v.cachedPass)
		v.cachedPass = nil
	}
}

func (v *VaultStore) getPassphrase() ([]byte, error) {
	if len(v.cachedPass) > 0 {
		return v.cachedPass, nil
	}
	if env := os.Getenv("SSHM_VAULT_PASSPHRASE"); env != "" {
		v.cachedPass = []byte(env)
		return v.cachedPass, nil
	}
	if v.passphraseFn == nil {
		return nil, errors.New("vault passphrase callback not configured (set SSHM_VAULT_PASSPHRASE)")
	}
	p, err := v.passphraseFn()
	if err != nil {
		return nil, err
	}
	v.cachedPass = p
	return v.cachedPass, nil
}

func (v *VaultStore) loadLocked() (map[string]string, error) {
	entries := map[string]string{}
	raw, err := os.ReadFile(v.path)
	if errors.Is(err, os.ErrNotExist) {
		return entries, nil
	}
	if err != nil {
		return nil, err
	}
	pass, err := v.getPassphrase()
	if err != nil {
		return nil, err
	}
	plain, err := DecryptBundle(raw, pass)
	if err != nil {
		return nil, err
	}
	defer ZeroBytes(plain)
	if err := json.Unmarshal(plain, &entries); err != nil {
		return nil, fmt.Errorf("vault JSON decode failed: %w", err)
	}
	return entries, nil
}

func writeAtomic(path string, data []byte, perm os.FileMode) error {
	tmpFile, err := os.CreateTemp(filepath.Dir(path), ".sshm-cred-*")
	if err != nil {
		return err
	}
	tmp := tmpFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmp)
		}
	}()
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	_ = os.Chmod(tmp, perm)
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(path)
		if err2 := os.Rename(tmp, path); err2 != nil {
			return err2
		}
	}
	cleanup = false
	return nil
}

func (v *VaultStore) saveLocked(entries map[string]string) error {
	pass, err := v.getPassphrase()
	if err != nil {
		return err
	}
	plain, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	defer ZeroBytes(plain)
	enc, err := EncryptBundle(plain, pass)
	if err != nil {
		return err
	}
	return writeAtomic(v.path, enc, 0o600)
}

func (v *VaultStore) Set(ref string, secret []byte) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	entries, err := v.loadLocked()
	if err != nil {
		return err
	}
	entries[ref] = string(secret)
	return v.saveLocked(entries)
}

func (v *VaultStore) Get(ref string) ([]byte, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	entries, err := v.loadLocked()
	if err != nil {
		return nil, err
	}
	val, ok := entries[ref]
	if !ok {
		return nil, ErrNotFound
	}
	return []byte(val), nil
}

func (v *VaultStore) Delete(ref string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	entries, err := v.loadLocked()
	if err != nil {
		return err
	}
	if _, ok := entries[ref]; !ok {
		return nil
	}
	delete(entries, ref)
	return v.saveLocked(entries)
}
