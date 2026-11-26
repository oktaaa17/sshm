package importer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oktaaa17/sshm/internal/config"
	"github.com/oktaaa17/sshm/internal/credstore"
)

func TestImportSSHConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ssh_config")
	sample := `
# Global wildcard should be skipped
Host *
    ServerAliveInterval 60

Host prod-api
    HostName 10.0.1.12
    User deploy
    Port 2222
    IdentityFile ~/.ssh/id_ed25519
    ProxyJump bastion

Host dev-box
    HostName 192.168.1.50
    User ubuntu
`
	if err := os.WriteFile(cfgPath, []byte(sample), 0o600); err != nil {
		t.Fatalf("write sample config: %v", err)
	}

	hosts, err := ImportSSHConfig(cfgPath, "imported")
	if err != nil {
		t.Fatalf("ImportSSHConfig failed: %v", err)
	}
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(hosts))
	}
	if hosts[0].Alias != "prod-api" || hosts[0].Port != 2222 || hosts[0].JumpHost != "bastion" || hosts[0].Group != "imported" {
		t.Errorf("unexpected host[0]: %+v", hosts[0])
	}
	if hosts[1].Alias != "dev-box" || hosts[1].Hostname != "192.168.1.50" || hosts[1].User != "ubuntu" {
		t.Errorf("unexpected host[1]: %+v", hosts[1])
	}
}

func TestEncryptedExportImportRoundTrip(t *testing.T) {
	srcDir := t.TempDir()
	srcMgr, err := config.Load(srcDir)
	if err != nil {
		t.Fatalf("Load src: %v", err)
	}
	srcStore := credstore.NewVaultStore(srcDir, func() ([]byte, error) { return []byte("vault-pw"), nil })

	saved, err := srcMgr.UpsertHost(config.Host{
		Alias:    "db-prod",
		Group:    "prod",
		Hostname: "10.0.0.99",
		User:     "postgres",
		Port:     5432,
	})
	if err != nil {
		t.Fatalf("UpsertHost: %v", err)
	}
	if err := srcStore.Set(saved.CredentialRef, []byte("secret-db-pass")); err != nil {
		t.Fatalf("Set secret: %v", err)
	}

	backupPath := filepath.Join(t.TempDir(), "backup.sshm.enc")
	if err := ExportEncrypted(srcMgr, srcStore, true, []byte("backup-pass"), backupPath); err != nil {
		t.Fatalf("ExportEncrypted failed: %v", err)
	}

	dstDir := t.TempDir()
	dstMgr, err := config.Load(dstDir)
	if err != nil {
		t.Fatalf("Load dst: %v", err)
	}
	dstStore := credstore.NewVaultStore(dstDir, func() ([]byte, error) { return []byte("dst-vault-pw"), nil })

	hCount, cCount, err := ImportEncrypted(dstMgr, dstStore, []byte("backup-pass"), backupPath)
	if err != nil {
		t.Fatalf("ImportEncrypted failed: %v", err)
	}
	if hCount != 1 || cCount != 1 {
		t.Fatalf("expected 1 host and 1 cred imported, got %d and %d", hCount, cCount)
	}

	h, _, ok := dstMgr.FindHost("db-prod")
	if !ok {
		t.Fatalf("db-prod missing after import")
	}
	gotPW, err := dstStore.Get(h.CredentialRef)
	if err != nil || string(gotPW) != "secret-db-pass" {
		t.Fatalf("expected restored password 'secret-db-pass', got %q (err=%v)", gotPW, err)
	}
}
