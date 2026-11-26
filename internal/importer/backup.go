package importer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/oktaaa17/sshm/internal/config"
	"github.com/oktaaa17/sshm/internal/credstore"
)

// BackupBundle holds an encrypted snapshot of sshm config, hosts, and optional credentials.
type BackupBundle struct {
	Version     int               `json:"version"`
	Config      config.AppConfig  `json:"config"`
	HostsFile   config.HostsFile  `json:"hosts_file"`
	Credentials map[string]string `json:"credentials,omitempty"`
}

// ExportEncrypted exports configuration, hosts, and optionally credentials to an AES-256-GCM encrypted file.
func ExportEncrypted(mgr *config.Manager, store credstore.Store, includePasswords bool, passphrase []byte, outPath string) error {
	bundle := BackupBundle{
		Version:   1,
		Config:    mgr.Config,
		HostsFile: mgr.HostsFile,
	}

	if includePasswords && store != nil {
		bundle.Credentials = map[string]string{}
		for _, h := range mgr.HostsFile.Hosts {
			if h.CredentialRef != "" {
				if pw, err := store.Get(h.CredentialRef); err == nil && len(pw) > 0 {
					bundle.Credentials[h.CredentialRef] = string(pw)
					credstore.ZeroBytes(pw)
				}
			}
			keyRef := credstore.KeyPassphraseRef(h.ID)
			if kp, err := store.Get(keyRef); err == nil && len(kp) > 0 {
				bundle.Credentials[keyRef] = string(kp)
				credstore.ZeroBytes(kp)
			}
		}
	}

	raw, err := json.Marshal(bundle)
	if err != nil {
		return err
	}
	defer credstore.ZeroBytes(raw)

	enc, err := credstore.EncryptBundle(raw, passphrase)
	if err != nil {
		return err
	}
	return os.WriteFile(outPath, enc, 0o600)
}

// ImportEncrypted decrypts and merges an exported .sshm.enc backup file.
func ImportEncrypted(mgr *config.Manager, store credstore.Store, passphrase []byte, inPath string) (int, int, error) {
	blob, err := os.ReadFile(filepath.Clean(inPath)) // #nosec G304 -- backup bundle path specified by user
	if err != nil {
		return 0, 0, err
	}
	plain, err := credstore.DecryptBundle(blob, passphrase)
	if err != nil {
		return 0, 0, err
	}
	defer credstore.ZeroBytes(plain)

	var bundle BackupBundle
	if err := json.Unmarshal(plain, &bundle); err != nil {
		return 0, 0, fmt.Errorf("failed to decode backup bundle: %w", err)
	}

	for gName, g := range bundle.HostsFile.Groups {
		if _, exists := mgr.HostsFile.Groups[gName]; !exists {
			mgr.HostsFile.Groups[gName] = g
		}
	}

	importedHosts := 0
	for _, h := range bundle.HostsFile.Hosts {
		if _, err := mgr.UpsertHost(h); err != nil {
			return importedHosts, 0, err
		}
		importedHosts++
	}

	importedCreds := 0
	if store != nil && len(bundle.Credentials) > 0 {
		for ref, secret := range bundle.Credentials {
			b := []byte(secret)
			if err := store.Set(ref, b); err == nil {
				importedCreds++
			}
			credstore.ZeroBytes(b)
		}
	}

	return importedHosts, importedCreds, nil
}
