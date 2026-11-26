# Changelog

All notable changes to **sshm** by **Oktanetflow** are documented in this file.

## [1.0.0] - 2026-11-26

### Added
- **Agile TUI Enhancements:**
  - Responsive **Split-Pane Host & Group Inspector** (auto-enabled on terminals ≥ 100 columns, toggleable with `i`).
  - **Async Live TCP Reachability & Latency Probes** with bounded worker pool (`r` to refresh, configurable via `ui.auto_probe_on_start`).
  - **Interactive Port Forward Manager (`f`)** to view, add (`-L`, `-R`, `-D`), and remove tunnels per host in the TUI.
  - **Inline Tags & Notes** fields in the Add/Edit host form, plus **Default Key** in the Group form.
  - **Quick Theme Switcher (`t`)** cycling `minimal-dark`, `minimal-light`, and `mono` with persistence.
  - **Header Summary Stats Bar** showing total hosts, groups, and live online count.
  - Persistent **`Copyright © 2026 Oktanetflow`** watermark across TUI, Help modal, CLI `--version`, Windows PE metadata, and installer.
- **Security & Windows Publisher Hardening:**
  - Owner-only Windows NTFS DACLs on `%APPDATA%\sshm\` (`internal/config/acl_windows.go`).
  - DPAPI buffer zeroing before `LocalFree` and `VaultStore.Close()` passphrase zeroing.
  - SOCKS5 10-second handshake deadline and non-loopback bind security warning.
  - Embedded Win32 PE `VersionInfo` (`Oktanetflow`), `asInvoker` manifest, and app icon via `goversioninfo`.
  - Automated security scan script (`scripts/security-scan.ps1`) and Authenticode signing script (`packaging/windows/sign.ps1`).
- **Distribution & Documentation:**
  - Per-user **Inno Setup 6 Windows EXE Installer** (`packaging/windows/sshm.iss`) with User `PATH` registration and zero UAC prompt.
  - GitHub Actions CI (`ci.yml`) and Release (`release.yml`) workflows with SHA-256 checksums and build provenance attestations.
  - Complete documentation suite (`README.md`, `SECURITY.md`, `CONTRIBUTING.md`, `LICENSE`, `docs/USAGE.md`, `docs/ARCHITECTURE.md`, `docs/WINDOWS_PUBLISHER_GUIDE.md`).

### Fixed
- Preserved `Tags`, `Notes`, `Pinned`, `LastConnected`, `Forwards`, and `CredentialRef` when editing hosts in the TUI.
- Preserved `DefaultIdentityFile` when editing groups in the TUI.
- Fixed `UpsertHost` alias match preserving existing `ID` and `CredentialRef` so saved passwords are never orphaned.
- Added mutex protection and unique-temp atomic file writes to prevent data races during `sshm exec @group --parallel`.
- Cleaned up key passphrases (`KeyPassphraseRef`) during `sshm group rm --delete-hosts`.
- Fixed bracketed IPv6 host and port-forward parsing (`[::1]:2222`).
- Added strict validation to `sshm config set`.
- Fixed TUI header/row padding alignment using `lipgloss.Width`.
