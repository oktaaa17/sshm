package cli

import (
	"testing"

	"github.com/oktaaa17/sshm/internal/config"
	"github.com/oktaaa17/sshm/internal/sshclient"
)

func TestResolveTargetHostIPv4AndIPv6(t *testing.T) {
	mgr, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	cases := []struct {
		target   string
		wantUser string
		wantHost string
		wantPort int
	}{
		{"root@10.0.0.5", "root", "10.0.0.5", 22},
		{"admin@10.0.0.5:2222", "admin", "10.0.0.5", 2222},
		{"ubuntu@[::1]:2200", "ubuntu", "::1", 2200},
		{"root@[2001:db8::1]", "root", "2001:db8::1", 22},
	}

	for _, tc := range cases {
		res, saved := resolveTargetHost(mgr, tc.target)
		if saved {
			t.Fatalf("expected ad-hoc target %q to not be saved", tc.target)
		}
		if res.EffectiveUser != tc.wantUser || res.Hostname != tc.wantHost || res.EffectivePort != tc.wantPort {
			t.Errorf("resolveTargetHost(%q) = %s@%s:%d, want %s@%s:%d",
				tc.target, res.EffectiveUser, res.Hostname, res.EffectivePort,
				tc.wantUser, tc.wantHost, tc.wantPort)
		}
	}
}

func TestApplyConfigSettingValidation(t *testing.T) {
	mgr, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if err := applyConfigSetting(mgr, "theme", "minimal-light"); err != nil {
		t.Fatalf("expected valid theme, got %v", err)
	}
	if err := applyConfigSetting(mgr, "theme", "neon-pink"); err == nil {
		t.Fatalf("expected invalid theme to return error")
	}
	if err := applyConfigSetting(mgr, "credential_backend", "plaintext"); err == nil {
		t.Fatalf("expected invalid credential_backend to return error")
	}
	if err := applyConfigSetting(mgr, "strict_host_key_checking", "maybe"); err == nil {
		t.Fatalf("expected invalid strict_host_key_checking to return error")
	}
	if err := applyConfigSetting(mgr, "keepalive_interval_sec", "-5"); err == nil {
		t.Fatalf("expected negative keepalive_interval_sec to return error")
	}
	if err := applyConfigSetting(mgr, "auto_probe_on_start", "false"); err != nil {
		t.Fatalf("expected valid auto_probe_on_start, got %v", err)
	}
	if mgr.Config.UI.AutoProbeOnStart {
		t.Fatalf("expected AutoProbeOnStart=false")
	}
}

func TestParseForwardSpecsIPv6(t *testing.T) {
	f, err := sshclient.ParseLocalOrRemoteSpec("local", "[::1]:8080:[2001:db8::2]:80")
	if err != nil {
		t.Fatalf("ParseLocalOrRemoteSpec IPv6 failed: %v", err)
	}
	if f.Bind != "[::1]:8080" || f.Target != "[2001:db8::2]:80" {
		t.Fatalf("unexpected forward parsed: %+v", f)
	}

	d, err := sshclient.ParseDynamicSpec("[::1]:1080")
	if err != nil {
		t.Fatalf("ParseDynamicSpec IPv6 failed: %v", err)
	}
	if d.Bind != "[::1]:1080" {
		t.Fatalf("unexpected dynamic forward: %+v", d)
	}

	if _, err := sshclient.ParseLocalOrRemoteSpec("local", "99999:127.0.0.1:80"); err == nil {
		t.Fatalf("expected out-of-range port to fail")
	}
}
