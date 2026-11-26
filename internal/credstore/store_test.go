package credstore

import (
	"bytes"
	"errors"
	"runtime"
	"testing"
)

func TestVaultStoreAndEncryptBundle(t *testing.T) {
	dir := t.TempDir()
	pass := []byte("super-secret-master-key")
	store := NewVaultStore(dir, func() ([]byte, error) {
		return pass, nil
	})

	ref := "sshm:TESTHOST01"
	secret := []byte("P@ssw0rd!2026")

	if err := store.Set(ref, secret); err != nil {
		t.Fatalf("Vault Set failed: %v", err)
	}

	got, err := store.Get(ref)
	if err != nil {
		t.Fatalf("Vault Get failed: %v", err)
	}
	if !bytes.Equal(got, secret) {
		t.Fatalf("secret mismatch: got %q want %q", got, secret)
	}

	// Wrong passphrase should fail decryption
	badStore := NewVaultStore(dir, func() ([]byte, error) {
		return []byte("wrong-passphrase"), nil
	})
	if _, err := badStore.Get(ref); err == nil {
		t.Fatalf("expected error with wrong passphrase")
	}

	// Delete should remove secret
	if err := store.Delete(ref); err != nil {
		t.Fatalf("Vault Delete failed: %v", err)
	}
	if _, err := store.Get(ref); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestWindowsBackends(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific credential backends")
	}

	t.Run("WinCred", func(t *testing.T) {
		store, err := New("wincred", t.TempDir(), nil)
		if err != nil {
			t.Fatalf("New wincred failed: %v", err)
		}
		ref := "sshm:unit-test-ephemeral-cred"
		t.Cleanup(func() { _ = store.Delete(ref) })

		secret := []byte("WinCredSecret#99")
		if err := store.Set(ref, secret); err != nil {
			t.Fatalf("WinCred Set failed: %v", err)
		}
		got, err := store.Get(ref)
		if err != nil {
			t.Fatalf("WinCred Get failed: %v", err)
		}
		if !bytes.Equal(got, secret) {
			t.Fatalf("WinCred mismatch: got %q want %q", got, secret)
		}
		if err := store.Delete(ref); err != nil {
			t.Fatalf("WinCred Delete failed: %v", err)
		}
		if _, err := store.Get(ref); !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound after WinCred delete, got %v", err)
		}
	})

	t.Run("DPAPI", func(t *testing.T) {
		dir := t.TempDir()
		store, err := New("dpapi", dir, nil)
		if err != nil {
			t.Fatalf("New dpapi failed: %v", err)
		}
		ref := "sshm:dpapi-test-host"
		secret := []byte("DPAPISecret!42")
		if err := store.Set(ref, secret); err != nil {
			t.Fatalf("DPAPI Set failed: %v", err)
		}
		got, err := store.Get(ref)
		if err != nil {
			t.Fatalf("DPAPI Get failed: %v", err)
		}
		if !bytes.Equal(got, secret) {
			t.Fatalf("DPAPI mismatch: got %q want %q", got, secret)
		}
		if err := store.Delete(ref); err != nil {
			t.Fatalf("DPAPI Delete failed: %v", err)
		}
		if _, err := store.Get(ref); !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound after DPAPI delete, got %v", err)
		}
	})
}
