# ADR-006: Independent Codebase & Per-User Windows Inno Setup Distribution

## Status
Accepted

## Date
2026-11-26

## Context
Two distribution and identity requirements shaped `sshm` v1.0.0:
1. **Clear Technical Provenance:** Another open-source project shares a similar name (`github.com/gu1llaum-3/sshm`), which wraps the external `ssh` binary and directly edits `~/.ssh/config`. Users and auditors must clearly understand that **Oktanetflow's `sshm`** (`github.com/oktaaa17/sshm`) is an independent, ground-up pure-Go SSH client & manager with a fundamentally different architecture.
2. **Frictionless Windows Installation:** Requiring Windows users to manually download a `.zip`, create a folder, and edit Windows System Environment Variables (`PATH`) causes high onboarding drop-off — yet requiring Administrator UAC elevation blocks corporate developers without local admin rights.

## Decision
1. **Independent Architecture & Transparent Branding:** Maintain `github.com/oktaaa17/sshm` as a clean greenfield MIT-licensed codebase copyrighted to **Oktanetflow**, explicitly documenting architectural differences in `docs/ARCHITECTURE.md` and embedding `Copyright © 2026 Oktanetflow` across CLI `--version`, TUI watermark, Win32 PE metadata, and installer screens.
2. **Dual Windows Release Distribution (`.github/workflows/release.yml` + `scripts/build-release.ps1`):**
   - **Per-User Inno Setup 6 `.exe` Installer (`packaging/windows/sshm.iss`):** Configured with `PrivilegesRequired=lowest`, installing into `%LOCALAPPDATA%\Programs\Oktanetflow\sshm`, automatically updating User `PATH` with zero UAC prompt, and registering a clean Add/Remove Programs uninstaller.
   - **Standalone Portable Binary:** `sshm-<version>-windows-amd64.exe` and `arm64` for portable/USB or scripted workflows.
3. **Verifiable Supply Chain:** Publish `SHA256SUMS.txt`, GitHub Artifact Attestations (`actions/attest-build-provenance`), and ready-to-enable Authenticode / Microsoft Trusted Signing (`packaging/windows/sign.ps1`).

## Alternatives Considered

### Forking `github.com/gu1llaum-3/sshm`
- **Pros:** Faster initial prototype.
- **Cons:** Incompatible core architecture (wraps external `ssh` binary instead of pure-Go SSHv2/SFTP/WinCred/DPAPI/NTFS ACLs/parallel `exec`).
- **Rejected:** A greenfield pure-Go architecture is cleaner, safer, and purpose-built for Windows.

### MSI / WiX Administrator Installer
- **Pros:** Enterprise Group Policy (GPO) deployment support.
- **Cons:** Heavy XML authoring, requires Admin UAC elevation for default `Program Files` installs, slower CI iteration.
- **Rejected:** Inno Setup 6 (`PrivilegesRequired=lowest`) provides instant per-user installs with zero UAC friction; MSI packaging can be added later if enterprise GPO demand arises.

### Shipping only a raw `.zip` archive
- **Pros:** Zero installer maintenance.
- **Cons:** Forces every Windows user to manually configure `PATH`.
- **Rejected:** Providing *both* a one-click per-user `.exe` installer and a portable standalone `.exe` serves both novice and power users.

## Consequences
- **Positive:** Non-admin Windows users install `sshm` and immediately run `sshm` from PowerShell or Windows Terminal in under 15 seconds.
- **Positive:** Every release is cryptographically verifiable via SHA-256 checksums, GitHub Artifact Attestations, and optional Authenticode signatures.
- **Negative:** Maintainers must keep `packaging/windows/sshm.iss`, `cmd/sshm/versioninfo.json`, and `.github/workflows/release.yml` version strings synchronized (automated via `-Version` parameters in `scripts/build-release.ps1` and tag-driven GitHub Actions).
