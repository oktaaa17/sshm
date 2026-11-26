# Spec: sshm by Oktanetflow — Windows-First CLI/TUI SSH Client

> **Product:** `sshm` (`sshm.exe`) — Minimalist & Agile CLI/TUI SSH Client for Windows  
> **Publisher / Copyright:** `Copyright © 2026 Oktanetflow`  
> **Repository Target:** `https://github.com/oktaaa17/sshm` (Go module: `github.com/oktaaa17/sshm`)  
> **Origin:** Independent pure-Go greenfield implementation (NOT a clone or fork of `github.com/gu1llaum-3/sshm`)  
> **Target Platform:** Windows 10 / 11 (`amd64`, `arm64`) first-class, with cross-platform Linux / macOS support  
> **Form Factor:** Single standalone zero-CGo binary (`sshm.exe`) + per-user Windows Inno Setup `.exe` installer

---

## 1. Objective

Deliver a production-ready, publisher-hardened, security-audited release of `sshm` branded for **Oktanetflow**, structured across four agile sprints:

1. **Agile UI Upgrade & Branding:** Upgrade the Bubble Tea TUI with richer operational details — a responsive **Split-Pane Host Inspector**, **async live TCP reachability & latency badges** (`● online 14ms` / `● offline` / `○ checking`), **inline Tags, Notes & Port Forward viewer/editor** in the Add/Edit form, a **Group Summary Stats bar**, a **quick Theme Switcher** (`t`), and persistent **`Copyright © 2026 Oktanetflow`** watermarking across the TUI, Help modal, CLI `--version`, Windows PE properties, and installer.
2. **Bug Fixes & Concurrency Hardening:** Eliminate all known data-loss, credential-orphaning, IPv6 parsing, Unicode width alignment, non-atomic file write, and parallel-execution race bugs.
3. **Security Audit & Windows Publisher Safety:** Pass automated Go vulnerability & static security scans (`go vet`, `go test -race`, `govulncheck`, `gosec`), enforce Windows owner-only NTFS ACLs on `%APPDATA%\sshm\`, zero sensitive memory buffers, embed Win32 PE VersionInfo + `asInvoker` manifest (zero UAC elevation, zero UPX packing), and ship ready-to-activate **Authenticode / Microsoft Trusted Signing** CI hooks + `SHA256SUMS.txt` + Microsoft Security Intelligence (WDSI) submission documentation so Windows Defender / SmartScreen never requires users to disable protection.
4. **Docs, Inno Setup EXE Installer & GitHub Publishing:** Ship complete documentation (`README.md`, `USAGE.md`, `SECURITY.md`, `CONTRIBUTING.md`, `CHANGELOG.md`, `LICENSE`), a per-user Inno Setup `.exe` installer (`sshm-setup-<version>-amd64.exe` that adds `sshm.exe` to user `PATH` without admin privileges), GitHub Actions CI + Release automation, and initialize/publish the GitHub repository.

---

## 2. Tech Stack

| Layer | Technology | Version / Notes |
| :--- | :--- | :--- |
| **Language** | Go (pure Go, `CGO_ENABLED=0`) | `go 1.26.0` (built with `go1.27.4`) |
| **SSHv2 / Crypto** | `golang.org/x/crypto` | `v0.59.0` (`ssh`, `knownhosts`, `argon2`, `AES-256-GCM`) |
| **SFTP** | `github.com/pkg/sftp` | `v1.13.13` |
| **Windows Agent Pipe** | `github.com/Microsoft/go-winio` | `v0.6.4` (`\\.\pipe\openssh-ssh-agent`) |
| **CLI Parser** | `github.com/spf13/cobra` | `v1.10.2` (subcommands + PowerShell completion) |
| **TUI Framework** | `github.com/charmbracelet/bubbletea` + `lipgloss` | `v1.3.10` / `v1.1.0` |
| **Windows APIs** | `golang.org/x/sys/windows` + `golang.org/x/term` | `v0.50.0` / `v0.48.0` (`advapi32`, `crypt32`, Win32 VT100, NTFS ACLs) |
| **Config Format** | `github.com/pelletier/go-toml/v2` | `v2.4.3` |
| **Win32 PE Metadata** | `github.com/josephspurrier/goversioninfo` (build tool) | Generates `cmd/sshm/resource_windows_amd64.syso` (`VersionInfo` + `asInvoker` manifest + icon) |
| **Security Scanners** | `golang.org/x/vuln/cmd/govulncheck`, `github.com/securego/gosec/v2/cmd/gosec` | Automated local + CI security gates |
| **Windows Installer** | Inno Setup 6 (`ISCC.exe`) | Per-user `PrivilegesRequired=lowest`, auto User `PATH`, uninstaller |
| **CI/CD & Signing** | GitHub Actions (`windows-latest`) | Reproducible `-trimpath` builds, `SHA256SUMS.txt`, optional Authenticode / Azure Trusted Signing |

---

## 3. Commands

```powershell
# 1. Generate Win32 PE VersionInfo + asInvoker manifest (.syso)
go generate ./cmd/sshm

# 2. Build reproducible Windows binary (NO UPX packing!)
$env:CGO_ENABLED="0"
go build -trimpath -ldflags "-s -w -X github.com/oktaaa17/sshm/internal/cli.Version=1.0.0" -o dist/sshm.exe ./cmd/sshm

# 3. Run unit & integration tests with race detector and coverage
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out

# 4. Static analysis & security scans
go vet ./...
govulncheck ./...
gosec -exclude-generated ./...

# 5. Build per-user Windows EXE Installer (Inno Setup)
iscc /DMyAppVersion=1.0.0 packaging/windows/sshm.iss

# 6. Generate SHA256 checksums for release artifacts
Get-FileHash dist/sshm.exe, dist/sshm-setup-1.0.0-amd64.exe -Algorithm SHA256
```

---

## 4. Project Structure

```text
cli-based-ssh-client-windows-support/
├── cmd/
│   └── sshm/
│       ├── main.go                    # Entry point + go:generate directive
│       ├── versioninfo.json           # Win32 PE metadata (Oktanetflow publisher + copyright)
│       ├── sshm.exe.manifest          # Windows asInvoker + UTF-8 + Win10/11 compat manifest
│       └── assets/sshm.ico            # Application icon embedded in PE & installer
├── internal/
│   ├── cli/                           # Cobra commands, IPv6 target parser, PowerShell completion
│   ├── config/                        # config.toml & hosts.toml, mutex-safe atomic writes, NTFS ACLs
│   ├── credstore/                     # WinCred, DPAPI (zeroed buffers), Argon2id+AES-256-GCM Vault
│   ├── sshclient/                     # Pure-Go SSHv2 dialer, ProxyJump, known_hosts, SFTP, forwards, TCP probe
│   ├── termwin/                       # Win32 VT100 raw console, live resize watcher
│   ├── tui/                           # Agile Bubble Tea TUI: list, split inspector, live probe, forms, watermark
│   └── importer/                      # ~/.ssh/config, PuTTY Registry, encrypted .sshm.enc backup
├── packaging/
│   └── windows/
│       ├── sshm.iss                   # Inno Setup 6 per-user installer script (adds User PATH)
│       └── sign.ps1                   # Authenticode / Microsoft Trusted Signing helper script
├── scripts/
│   ├── build-release.ps1              # End-to-end local release builder (syso + exe + installer + SHA256)
│   └── security-scan.ps1              # Runs go vet, go test -race, govulncheck, gosec
├── docs/
│   ├── ARCHITECTURE.md                # Tech stack, design principles, comparison vs gu1llaum-3/sshm
│   ├── WINDOWS_PUBLISHER_GUIDE.md     # Trusted Signing, Authenticode, SmartScreen & Microsoft WDSI guide
│   └── USAGE.md                       # Complete CLI & TUI user manual
├── .github/
│   └── workflows/
│       ├── ci.yml                     # Build, test -race, vet, govulncheck, gosec on Windows/Linux
│       └── release.yml                # Tag release -> build exe + Inno Setup installer + optional sign + SHA256 + GitHub Release
├── SPEC.md                            # Living specification (this file)
├── README.md                          # Project overview, install options, screenshots/layout, Oktanetflow copyright
├── SECURITY.md                        # Vulnerability disclosure, threat model, Windows SmartScreen verification
├── CONTRIBUTING.md                    # Agile workflow & PR standards
├── CHANGELOG.md                       # Release history
├── LICENSE                            # MIT License — Copyright © 2026 Oktanetflow
└── go.mod / go.sum
```

---

## 5. Detailed Agile Feature Requirements

### 5.1 Agile TUI Layout & Watermark (`internal/tui`)

```text
 sshm v1.0.0  ● 12 hosts · 3 groups · 9 online    [?] Help [a] Add [g] Group [t] Theme [r] Probe [q] Quit
 ────────────────────────────────────────────────────────────────────────────────────────────────────────
 > search: _                                               [All] prod  staging  personal

 ▼ prod (3)                                          │ Host Inspector — prod-api-01
   ● prod-api-01   root@10.0.1.12    [pw]  ● 14ms    │ ──────────────────────────────────────────────
   ● prod-api-02   root@10.0.1.13    [key] ● 22ms    │ Endpoint  : root@10.0.1.12:22
   ● prod-db-01    pg@10.0.1.20:2222 [pw]  ● 31ms    │ Group     : prod (red badge)
                                                     │ Auth      : password (Windows Credential Mgr)
 ▼ staging (2)                                       │ Jump Host : bastion-prod (inherited)
   ● stg-web       ubuntu@10.0.2.5   [pw]  ● 45ms    │ Status    : Online (14ms TCP)
   ● stg-worker    ubuntu@10.0.2.6   [pw]  ○ off     │ Tags      : #api #linux
                                                     │ Forwards  : L 127.0.0.1:8080 -> 127.0.0.1:80
                                                     │ Notes     : Primary production API node
                                                     │ Last Seen : 2m ago
 ────────────────────────────────────────────────────────────────────────────────────────────────────────
 [Enter] Connect [Tab] Group [/] Search [i] Inspector [e] Edit [f] Forwards [d] Delete [p] Pin [r] Probe
 Copyright © 2026 Oktanetflow  ·  Theme: minimal-dark  ·  Store: Windows Credential Manager
```

* **Responsive Layout:** Split-pane Host Inspector appears automatically when terminal width ≥ 100 columns and can be toggled manually with `i`. On narrow terminals (< 100 cols), falls back cleanly to single-column list + compact detail footer.
* **Async TCP Reachability Probe:** Non-blocking Bubble Tea `tea.Cmd` worker pool (bounded concurrency, short configurable timeout e.g. `1500ms`) checks TCP `host:port` reachability on startup and on `[r]` refresh, displaying green latency (`● 14ms`), amber (`○ ...`), or red (`● off`). Configurable via `ui.auto_probe_on_start` (default `true`).
* **Extended Host Metadata:** Add `Notes string` (`toml:"notes,omitempty"`) to `config.Host`. Add/Edit form preserves all existing fields (`Tags`, `Notes`, `Pinned`, `LastConnected`, `Forwards`, `CredentialRef`) and allows inline editing of `Tags` (comma-separated) and `Notes`.
* **Interactive Port Forward Modal (`f`):** View, add (`-L`, `-R`, `-D`), and remove persistent port forwards on the selected host right inside the TUI.
* **Quick Theme Switcher (`t`):** Cycles `minimal-dark` → `minimal-light` → `mono` live and persists to `config.toml`.
* **Persistent Watermark:** `Copyright © 2026 Oktanetflow` displayed in the TUI status/copyright bar, Help modal, CLI `sshm --version`, Windows PE `LegalCopyright` / `CompanyName`, and Inno Setup installer footer.

### 5.2 Bug Fixes & Concurrency Hardening

1. Preserve `Tags`, `Notes`, `Pinned`, `LastConnected`, `Forwards`, `CredentialRef` when editing hosts in TUI [`saveHostForm`](file:///c:/Users/dimas/cli-based-ssh-client-windows-support/internal/tui/model.go#L546-L599).
2. Preserve `DefaultIdentityFile` (and expose it for editing) in TUI [`updateGroup`](file:///c:/Users/dimas/cli-based-ssh-client-windows-support/internal/tui/model.go#L642-L712).
3. Fix [`UpsertHost`](file:///c:/Users/dimas/cli-based-ssh-client-windows-support/internal/config/config.go#L408-L461) so alias match preserves the existing `ID` and `CredentialRef` instead of overwriting `CredentialRef` with a freshly generated ID.
4. Add `sync.Mutex` inside `config.Manager` + unique temp filenames (`os.CreateTemp`) in `writeFileAtomic` so concurrent `sshm exec @group --parallel` calls to `TouchLastConnected` are completely race-free.
5. Delete `credstore.KeyPassphraseRef(h.ID)` during `sshm group rm --delete-hosts`.
6. Fix bracketed IPv6 parsing in `resolveTargetHost` (`net.SplitHostPort`) and `ParseLocalOrRemoteSpec`.
7. Use `lipgloss.Width` / `runewidth` for all TUI width, truncation, and right-padding calculations.
8. Validate `config set` enum values (`credential_backend`, `strict_host_key_checking`, `theme`) and positive integer bounds (`keepalive_interval_sec`).
9. Make `DPAPIStore.saveMap`, `VaultStore.saveLocked`, and `knownhosts.RemoveHostKey` atomic via temp-file + rename.

### 5.3 Security & Windows Publisher Hardening

1. **Windows NTFS Owner-Only ACLs:** Implement `internal/config/acl_windows.go` using `golang.org/x/sys/windows` (`SetNamedSecurityInfo` / `BuildExplicitAccessWithName`) to lock `%APPDATA%\sshm\` and sensitive files to the current Windows user SID + `SYSTEM` only (disabling inherited broad permissions).
2. **Memory & Network Hygiene:** Zero DPAPI `outBlob` memory before `LocalFree`; add `Close()` on `VaultStore` to zero `cachedPass`; enforce 10-second handshake `SetDeadline` in `handleSOCKS5Conn` and warn if SOCKS5 binds to a non-loopback address without auth.
3. **Static & Dependency Scans:** Zero critical/high findings from `go vet ./...`, `govulncheck ./...`, and `gosec ./...`.
4. **Windows Publisher Trust (No Need to Turn Off Windows Security):**
   * Embed Win32 PE `VersionInfo` (`CompanyName: Oktanetflow`, `ProductName: sshm`, `LegalCopyright: Copyright © 2026 Oktanetflow`) + `sshm.exe.manifest` (`requestedExecutionLevel level="asInvoker"`, UTF-8 activeCodePage, Windows 10/11 compatibility GUIDs) + `.ico` icon via `goversioninfo`.
   * **Never** use UPX or executable packers (major Windows Defender false-positive trigger).
   * Compile reproducibly with `-trimpath` and publish `SHA256SUMS.txt`.
   * Provide `packaging/windows/sign.ps1` and GitHub Actions signing step supporting both **Microsoft Trusted Signing** (`azure/trusted-signing-action`) and standard **Authenticode `.pfx` (`signtool.exe` + RFC 3161 timestamp)**.
   * Document Microsoft Security Intelligence (WDSI) submission workflow in `docs/WINDOWS_PUBLISHER_GUIDE.md`.

---

## 6. Code Style

Follow idiomatic Go (`gofmt`), explicit error wrapping (`%w`), zero CGo, and Windows-safe build tags (`*_windows.go` / `*_other.go`).

```go
// Example: Atomic file persistence with unique temp file and Windows-safe replace.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, ".sshm-tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", path, err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return err
	}
	return replaceFile(tmpPath, path)
}
```

---

## 7. Testing Strategy

* **Unit & Concurrency Tests (`go test -race ./...`):**
  * `internal/config`: group inheritance, alias upsert credential-ref preservation, concurrent `TouchLastConnected` race test, Windows NTFS ACL verification.
  * `internal/credstore`: WinCred, DPAPI, Vault round-trip + atomic write + `Close()` zeroing.
  * `internal/sshclient`: embedded mock SSH server tests (password/key auth, TOFU/mismatch, ProxyJump, IPv6 forward spec parsing, SOCKS5 timeout, TCP reachability probe).
  * `internal/tui`: Bubble Tea state-machine tests (edit preserves tags/notes/pinned/forwards, inspector toggle, theme cycling, probe message handling, watermark rendering, wide Unicode padding).
  * `internal/cli`: target parsing (including IPv6), `config set` validation, remote path parsing.
* **Security Gates:** `go vet ./...`, `govulncheck ./...`, `gosec ./...`.
* **Packaging Verification:** Build `dist/sshm.exe` with embedded PE metadata (`(Get-Item dist\sshm.exe).VersionInfo`), build `dist/sshm-setup-<version>-amd64.exe` with Inno Setup (if available locally or verified via syntax + CI), verify SHA256 hashes.

---

## 8. Boundaries

* **Always do:**
  * Run `go test -race ./...` and `go vet ./...` after every sprint/task before proceeding.
  * Preserve all existing comments/docstrings unrelated to changes.
  * Keep secrets out of TOML files, logs, git commits, and CLI argv.
  * Maintain `Copyright © 2026 Oktanetflow` across UI, PE metadata, docs, and installer.
* **Ask first:**
  * Pushing to a remote GitHub repository or creating a public GitHub repo (requires your GitHub auth/confirmation).
  * Installing third-party system packages (e.g., Inno Setup / `gh` CLI via `winget`) if not already present.
  * Adding new production Go runtime dependencies beyond the vetted list in Section 2.
* **Never do:**
  * Never pack executables with UPX or use any AV/SmartScreen evasion technique.
  * Never disable Windows Defender or instruct users to turn off Windows Security.
  * Never commit private keys, `.pfx` certificates, passwords, or `*.sshm.enc` backups to git.
  * Never remove or skip failing tests.

---

## 9. Success Criteria

1. `go test -race ./...` passes across all packages including new regression tests for all 9 bugs and new TUI/ACL/probe features.
2. `go vet ./...`, `govulncheck ./...`, and `gosec ./...` report zero unresolved high/critical issues.
3. Launching `sshm.exe` displays the upgraded TUI with Split-Pane Host Inspector, async online/latency indicators, Tags/Notes/Forwards editing, Theme Switcher, Group Stats bar, and `Copyright © 2026 Oktanetflow` watermark.
4. `(Get-Item dist\sshm.exe).VersionInfo` reports `CompanyName = "Oktanetflow"`, `LegalCopyright = "Copyright © 2026 Oktanetflow"`, `ProductName = "sshm"`, and embedded `asInvoker` manifest.
5. Inno Setup script `packaging/windows/sshm.iss` compiles cleanly into `dist/sshm-setup-1.0.0-amd64.exe` (per-user install, no UAC prompt, User `PATH` registration, clean uninstall) and `SHA256SUMS.txt` is generated.
6. Complete docs (`README.md`, `USAGE.md`, `SECURITY.md`, `CONTRIBUTING.md`, `CHANGELOG.md`, `LICENSE`, `docs/ARCHITECTURE.md`, `docs/WINDOWS_PUBLISHER_GUIDE.md`) and `.github/workflows/{ci,release}.yml` are committed cleanly to Git and ready for (or pushed to) `oktaaa17/sshm`.
