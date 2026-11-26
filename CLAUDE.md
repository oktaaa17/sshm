# CLAUDE.md — Project Rules & Agent Context for `sshm` (Oktanetflow)

> **Publisher:** Oktanetflow · **Copyright © 2026 Oktanetflow**  
> **Repository:** `https://github.com/oktaaa17/sshm`  
> **Module:** `github.com/oktaaa17/sshm`

This file provides essential architectural context, commands, and non-negotiable invariants for engineers and AI coding agents working in this repository.

---

## Project Overview

`sshm` is a pure-Go (`CGO_ENABLED=0`), single-binary CLI & TUI SSH client engineered first for **Windows 10/11** (`amd64`/`arm64`) with cross-platform Linux/macOS support. It is an independent greenfield codebase (not a fork of `gu1llaum-3/sshm`).

### Authoritative References
- **Product Spec:** [SPEC.md](SPEC.md)
- **Documentation Spec:** [docs/SPEC_DOCUMENTATION.md](docs/SPEC_DOCUMENTATION.md)
- **Architecture & Package Map:** [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- **Architecture Decision Records (Read Before Changing Core Design):** [docs/decisions/README.md](docs/decisions/README.md)
- **Security Policy:** [SECURITY.md](SECURITY.md)
- **Operational & Contribution Guidelines:** [docs/GUIDELINES.md](docs/GUIDELINES.md)

---

## Essential Commands

```powershell
# Run unit & concurrency race tests
go test -race ./...

# Run full local security & quality gate (go vet, go test -race, govulncheck, gosec)
powershell -ExecutionPolicy Bypass -File scripts/security-scan.ps1

# Generate Win32 PE VersionInfo + asInvoker manifest (.syso)
go generate ./cmd/sshm

# Build reproducible Windows binary (CGO_ENABLED=0, NO UPX!)
$env:CGO_ENABLED="0"
go build -trimpath -ldflags "-s -w -X github.com/oktaaa17/sshm/internal/cli.Version=1.0.0" -o dist/sshm.exe ./cmd/sshm

# End-to-end local release build (binary + Inno Setup installer + SHA256SUMS)
powershell -ExecutionPolicy Bypass -File scripts/build-release.ps1 -Version 1.0.0

# Regenerate TUI visual mockups (.png) and illustrated how-to demo videos (.gif + .webm)
node docs/scripts/render-media.mjs

# Verify documentation links, ADRs, and binary media integrity
powershell -ExecutionPolicy Bypass -File docs/scripts/verify-docs.ps1
```

---

## Codebase Map

| Path | Purpose |
| :--- | :--- |
| `cmd/sshm/` | Entrypoint, `versioninfo.json`, `sshm.exe.manifest` (`asInvoker`), `assets/sshm.ico` |
| `internal/cli/` | Cobra subcommands (`connect`, `ls`, `add`, `edit`, `rm`, `group`, `exec`, `cp`, `import`, `export`, `hostkey`, `config`), IPv4/IPv6 target parser |
| `internal/tui/` | Bubble Tea TUI (`model.go`, `inspector.go`, `forwards.go`, `styles.go`) with split-pane inspector, async TCP probes, 3 themes, Oktanetflow watermark |
| `internal/config/` | Mutex-safe TOML config/hosts manager, group inheritance, unique-temp atomic persistence, Win32 NTFS owner-only DACLs (`acl_windows.go`) |
| `internal/credstore/` | 3-tier `Store`: Windows Credential Manager (`wincred`), DPAPI (`dpapi`), Argon2id + AES-256-GCM Vault (`vault`) |
| `internal/sshclient/` | Pure-Go SSHv2 dialer, cycle-checked `ProxyJump`, `known_hosts` TOFU/mismatch, SFTP `cp`, `-L`/`-R`/`-D` SOCKS5, bounded TCP probe |
| `internal/termwin/` | Win32 VT100 raw console handling & 150ms live resize polling |
| `internal/importer/` | `~/.ssh/config`, PuTTY Registry, and encrypted `.sshm.enc` backup/restore |
| `packaging/windows/` | Inno Setup 6 per-user installer (`sshm.iss`) and Authenticode signing helper (`sign.ps1`) |
| `docs/` | User manual, ADRs (`docs/decisions/`), guidelines, use cases, mockups, videos, and illustrations |

---

## Non-Negotiable Invariants & Known Gotchas

1. **Zero CGo (`CGO_ENABLED=0`):** Never add CGo dependencies. Use `golang.org/x/sys/windows` with `*_windows.go` / `*_other.go` build tags.
2. **Zero Plaintext Secrets:** Never store passwords or key passphrases in `hosts.toml`, logs, CLI argv, or git commits. Always route through `internal/credstore.Store` and zero sensitive byte slices after use.
3. **Preserve Unedited Fields:** Any TUI or CLI host/group edit must preserve `ID`, `CredentialRef`, `Tags`, `Notes`, `Pinned`, `LastConnected`, `Forwards`, and `DefaultIdentityFile` (see [ADR-002](docs/decisions/ADR-002-save-password-by-default-and-credstore.md) & [ADR-005](docs/decisions/ADR-005-toml-config-group-inheritance-and-atomic-writes.md)).
4. **Atomic Writes & Mutexes:** All persistence (`config.toml`, `hosts.toml`, DPAPI map, vault, `known_hosts`) must use unique temp files + atomic replace so `sshm exec @group --parallel` remains race-free.
5. **Windows Terminal & Unicode Width:** Always strip Windows NUL modifier runes in TUI inputs and measure terminal string widths using `lipgloss.Width` (never `len(s)`).
6. **Windows Security Compliance:** Never pack binaries with UPX, never request UAC elevation (`asInvoker` only), and never instruct users to disable Windows Defender or SmartScreen (see [ADR-003](docs/decisions/ADR-003-windows-first-vt100-ntfs-acl-and-pe-packaging.md)).
7. **ADR Discipline:** Any significant architectural change requires a new sequentially numbered ADR in `docs/decisions/` — never delete historical ADRs.
