package config

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestConfigLoadDefaultsAndGroupInheritance(t *testing.T) {
	dir := t.TempDir()
	mgr, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !mgr.Config.General.SavePasswordByDefault {
		t.Fatalf("expected save_password_by_default to be true by default")
	}

	// Create group with defaults
	err = mgr.CreateOrUpdateGroup("prod", Group{
		Color:       "red",
		DefaultUser: "deploy",
		DefaultPort: 2222,
		DefaultJump: "bastion",
	})
	if err != nil {
		t.Fatalf("CreateOrUpdateGroup failed: %v", err)
	}

	// Add host without specifying user/port/jump -> should inherit from prod
	saved, err := mgr.UpsertHost(Host{
		Alias:    "api-01",
		Group:    "prod",
		Hostname: "10.0.0.10",
	})
	if err != nil {
		t.Fatalf("UpsertHost failed: %v", err)
	}
	if saved.CredentialRef == "" {
		t.Fatalf("expected non-empty credential_ref")
	}

	// Reload from disk to verify persistence
	mgr2, err := Load(dir)
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	h, _, ok := mgr2.FindHost("API-01") // case-insensitive lookup
	if !ok {
		t.Fatalf("expected to find host api-01 after reload")
	}

	res := mgr2.ResolveHost(h)
	if res.EffectiveUser != "deploy" {
		t.Errorf("expected inherited user 'deploy', got %q", res.EffectiveUser)
	}
	if res.EffectivePort != 2222 {
		t.Errorf("expected inherited port 2222, got %d", res.EffectivePort)
	}
	if res.EffectiveJumpHost != "bastion" {
		t.Errorf("expected inherited jump 'bastion', got %q", res.EffectiveJumpHost)
	}
	if res.GroupColor != "red" {
		t.Errorf("expected group color 'red', got %q", res.GroupColor)
	}
	if !res.EffectiveSavePassword {
		t.Errorf("expected EffectiveSavePassword=true")
	}

	// Verify hosts.toml has no plaintext password field (save_password is a boolean setting, not a password)
	raw, err := os.ReadFile(mgr2.HostsPath())
	if err != nil {
		t.Fatalf("read hosts.toml: %v", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "password =") || strings.HasPrefix(trimmed, "password=") {
			t.Errorf("hosts.toml must never contain plaintext password field: %s", trimmed)
		}
	}
}

func TestGroupRenameAndDelete(t *testing.T) {
	dir := t.TempDir()
	mgr, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	_, err = mgr.UpsertHost(Host{Alias: "web1", Group: "staging", Hostname: "192.168.1.5"})
	if err != nil {
		t.Fatalf("UpsertHost failed: %v", err)
	}
	if err := mgr.RenameGroup("staging", "stg"); err != nil {
		t.Fatalf("RenameGroup failed: %v", err)
	}
	h, _, ok := mgr.FindHost("web1")
	if !ok || h.Group != "stg" {
		t.Fatalf("expected host group to be 'stg', got %q", h.Group)
	}

	// Delete group without deleting hosts -> moves host to default
	removed, err := mgr.DeleteGroup("stg", false)
	if err != nil {
		t.Fatalf("DeleteGroup failed: %v", err)
	}
	if len(removed) != 0 {
		t.Fatalf("expected 0 removed hosts, got %d", len(removed))
	}
	h, _, _ = mgr.FindHost("web1")
	if h.Group != DefaultGroupName {
		t.Fatalf("expected host moved to %q, got %q", DefaultGroupName, h.Group)
	}
}

func TestUpsertHostPreservesCredentialRefAndID(t *testing.T) {
	dir := t.TempDir()
	mgr, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	orig, err := mgr.UpsertHost(Host{
		Alias:    "prod-api",
		Hostname: "10.0.0.1",
		Group:    "prod",
		Notes:    "Primary node",
	})
	if err != nil {
		t.Fatalf("first UpsertHost failed: %v", err)
	}

	// Upsert again with same alias (case-insensitive) and empty ID/CredentialRef -> must preserve existing ID and CredentialRef!
	updated, err := mgr.UpsertHost(Host{
		Alias:    "PROD-API",
		Hostname: "10.0.0.2",
		Group:    "prod",
		Notes:    "Updated node",
	})
	if err != nil {
		t.Fatalf("second UpsertHost failed: %v", err)
	}

	if updated.ID != orig.ID {
		t.Fatalf("expected ID %q to be preserved, got %q", orig.ID, updated.ID)
	}
	if updated.CredentialRef != orig.CredentialRef {
		t.Fatalf("expected CredentialRef %q to be preserved, got %q", orig.CredentialRef, updated.CredentialRef)
	}
	if updated.Notes != "Updated node" {
		t.Fatalf("expected Notes updated, got %q", updated.Notes)
	}
}

func TestConcurrentTouchLastConnected(t *testing.T) {
	dir := t.TempDir()
	mgr, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	for i := 0; i < 6; i++ {
		_, err := mgr.UpsertHost(Host{
			Alias:    fmt.Sprintf("node-%d", i),
			Hostname: fmt.Sprintf("10.0.0.%d", i+1),
		})
		if err != nil {
			t.Fatalf("UpsertHost failed: %v", err)
		}
	}

	var wg sync.WaitGroup
	errs := make(chan error, 18)
	for round := 0; round < 3; round++ {
		for i := 0; i < 6; i++ {
			wg.Add(1)
			alias := fmt.Sprintf("node-%d", i)
			go func(a string) {
				defer wg.Done()
				if err := mgr.TouchLastConnected(a); err != nil {
					errs <- err
				}
			}(alias)
		}
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("concurrent TouchLastConnected failed: %v", err)
	}
}

func TestRestrictDirToCurrentUser(t *testing.T) {
	dir := t.TempDir()
	if err := RestrictDirToCurrentUser(dir); err != nil {
		t.Fatalf("RestrictDirToCurrentUser failed: %v", err)
	}
	// Directory must remain accessible to the current user after ACL hardening.
	mgr, err := Load(dir)
	if err != nil {
		t.Fatalf("Load after ACL hardening failed: %v", err)
	}
	if _, err := mgr.UpsertHost(Host{Alias: "acl-test", Hostname: "127.0.0.1"}); err != nil {
		t.Fatalf("UpsertHost after ACL hardening failed: %v", err)
	}
}
