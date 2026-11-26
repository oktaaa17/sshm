# Architecture & Tech Stack — sshm by Oktanetflow

> **Copyright © 2026 Oktanetflow**

## 1. Architectural Overview

`sshm` is a pure-Go (`CGO_ENABLED=0`), single-binary CLI & TUI SSH client designed with Windows 10/11 as a first-class platform. Unlike wrapper tools that shell out to `ssh.exe`, `sshm` embeds a complete native SSHv2 and SFTP protocol stack, native Win32 VT100 terminal management, and OS-native credential vaults.

```mermaid
flowchart TD
    Entry["cmd/sshm/main.go (PE Metadata + asInvoker Manifest)"] --> CLI["internal/cli (Cobra Commands & Completion)"]
    CLI --> TUI["internal/tui (Bubble Tea Agile TUI + Split Inspector + Watermark)"]
    CLI --> SSH["internal/sshclient (Pure-Go SSHv2, ProxyJump, SFTP, -L/-R/-D, TCP Probe)"]
    TUI --> SSH
    TUI --> CFG["internal/config (TOML Store, Group Inheritance, Mutex Atomic Writes, Win32 NTFS ACLs)"]
    CLI --> CFG
    CLI --> IMP["internal/importer (~/.ssh/config, PuTTY Registry, AES-256-GCM Backups)"]
    CLI --> CRED["internal/credstore (WinCred / DPAPI / Argon2id+AES-256-GCM Vault)"]
    TUI --> CRED
    SSH --> CRED
    SSH --> TERM["internal/termwin (Win32 VT100 Console Mode & Live Resize Watcher)"]
```

---

## 2. Package Responsibilities

| Package | Responsibility |
| :--- | :--- |
| `cmd/sshm` | Binary entry point, `go:generate` Win32 PE resource embedding (`versioninfo.json`, `sshm.exe.manifest`, `assets/sshm.ico`). |
| `internal/cli` | Cobra subcommands (`connect`, `ls`, `add`, `edit`, `rm`, `group`, `exec`, `cp`, `import`, `export`, `hostkey`, `config`), IPv4/IPv6 ad-hoc target parser, PowerShell completion. |
| `internal/tui` | Bubble Tea Elm-architecture TUI (`model.go`, `inspector.go`, `forwards.go`, `styles.go`): collapsible groups, fuzzy search, split-pane inspector, async TCP latency probes, port-forward manager, theme switcher, and Oktanetflow watermark. |
| `internal/config` | Thread-safe TOML config/host manager, group-default inheritance, unique-temp atomic persistence, and Windows NTFS owner-only DACL hardening (`acl_windows.go`). |
| `internal/credstore` | Pluggable `Store` interface with 3 backends: Windows Credential Manager (`advapi32.dll`), Windows DPAPI (`crypt32.dll`), and portable `Argon2id` + `AES-256-GCM` vault. |
| `internal/sshclient` | Pure-Go `golang.org/x/crypto/ssh` client, `ProxyJump` cycle-checked chaining, OpenSSH `known_hosts` TOFU/mismatch protection, keep-alive, SFTP copy, RFC 1928 SOCKS5 + `-L/-R` tunneling, and bounded TCP prober (`probe.go`). |
| `internal/termwin` | Native Win32 Console VT100 processing (`ENABLE_VIRTUAL_TERMINAL_PROCESSING` / `ENABLE_VIRTUAL_TERMINAL_INPUT`), raw mode restoration, and 150ms resize polling. |
| `internal/importer` | OpenSSH `~/.ssh/config` parser, Windows PuTTY Registry (`HKCU\Software\SimonTatham\PuTTY\Sessions`) importer, and encrypted `.sshm.enc` export/import. |

---

## 3. Comparison vs `github.com/gu1llaum-3/sshm`

`sshm` by Oktanetflow is **not a clone or fork** of `gu1llaum-3/sshm`. It is a ground-up independent Go codebase tailored for Windows-first operations:

| Capability | Oktanetflow `sshm` | `gu1llaum-3/sshm` |
| :--- | :--- | :--- |
| **SSH Engine** | Built-in pure-Go `x/crypto/ssh` + SFTP + Win32 VT100 PTY | Executes external system `ssh` binary |
| **Config Store** | `%APPDATA%\sshm\config.toml` & `hosts.toml` + group defaults + importers | Native `~/.ssh/config` (`Include`) directly |
| **Password Security** | Save-password-by-default via Windows Credential Manager / DPAPI / AES-256-GCM vault + Win32 NTFS ACLs | Delegates to system OpenSSH keys/agent |
| **Batch & File Ops** | Parallel `@group` `exec` + built-in SFTP `cp` + encrypted backups | Focused on interactive SSH + port forwarding |
| **Windows Packaging** | Win32 PE metadata, `asInvoker` manifest, per-user Inno Setup `.exe` installer | Cross-platform binary |
