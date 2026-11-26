# ADR-003: Native Win32 VT100 PTY, NTFS Owner-Only ACLs & Non-Packed PE Packaging

## Status
Accepted

## Date
2026-11-26

## Context
CLI/TUI tools ported casually from Linux to Windows often suffer from four severe failure modes:
1. Broken ANSI escape rendering, stuck raw terminal states, or missing window-resize (`SIGWINCH` does not exist natively on Windows) inside Windows Terminal, PowerShell 5.1/7+, CMD, and VS Code.
2. POSIX `os.Chmod(path, 0600)` is largely a no-op on Windows NTFS, leaving `%APPDATA%\sshm\` readable by other local accounts or broad inherited ACL groups.
3. Unsigned or UPX-packed Go `.exe` binaries frequently trigger Windows Defender heuristic false positives or unnecessary UAC Administrator prompts, causing users to distrust the tool.

## Decision
Treat Windows 10/11 internals as first-class engineering requirements:
1. **Native Win32 Console Handling (`internal/termwin/term_windows.go`):** Enable `ENABLE_VIRTUAL_TERMINAL_PROCESSING` and `ENABLE_VIRTUAL_TERMINAL_INPUT`, safely restore console mode on exit/panic, strip Windows NUL modifier runes in TUI forms, and run a 150ms console size polling watcher to transmit SSH `window-change` requests seamlessly.
2. **NTFS Owner-Only DACLs (`internal/config/acl_windows.go`):** Lock `%APPDATA%\sshm\` and sensitive config/vault files using Win32 `SetNamedSecurityInfo` (`PROTECTED_DACL_SECURITY_INFORMATION`) granting access strictly to the current user's SID and `NT AUTHORITY\SYSTEM`.
3. **Publisher-Hardened Win32 PE Packaging (`cmd/sshm/`):**
   - Embed Win32 `VersionInfo` (`CompanyName: Oktanetflow`, `ProductName: sshm`, `LegalCopyright: Copyright © 2026 Oktanetflow`) and application icon via `goversioninfo` (`versioninfo.json`).
   - Embed `sshm.exe.manifest` declaring `requestedExecutionLevel level="asInvoker"` (zero UAC admin elevation), UTF-8 `activeCodePage`, and Windows 10/11 compatibility GUIDs.
   - Compile with `-trimpath` and **strictly forbid UPX or executable packers**.
   - Provide Authenticode / Microsoft Trusted Signing hooks (`packaging/windows/sign.ps1`, `.github/workflows/release.yml`), `SHA256SUMS.txt`, GitHub Artifact Attestations, and Microsoft WDSI submission guidance (`docs/WINDOWS_PUBLISHER_GUIDE.md`).

## Alternatives Considered

### Relying solely on `os.Chmod(0600)` on Windows
- **Pros:** Zero platform-specific code.
- **Cons:** Leaves inherited NTFS read permissions intact on shared or domain-joined Windows machines.
- **Rejected:** Unacceptable for an SSH manager storing vault files and host topologies.

### Compressing `sshm.exe` with UPX to reduce binary size
- **Pros:** Shrinks the ~10 MB executable to ~3–4 MB.
- **Cons:** UPX stubs are one of the highest-frequency heuristic triggers in Windows Defender and enterprise EDR products; users get false-positive malware alerts.
- **Rejected:** Binary size savings (~6 MB) are negligible compared to trust and Defender cleanliness.

### Requesting Administrator (`requireAdministrator`) in manifest / installer
- **Pros:** Allows installing into `C:\Program Files\`.
- **Cons:** Triggers UAC prompts on every install/update; blocks non-admin enterprise developers from using `sshm`.
- **Rejected:** Least-privilege `asInvoker` execution + per-user `%LOCALAPPDATA%` install works for every Windows user without admin rights.

## Consequences
- **Positive:** `sshm` runs cleanly across Windows Terminal, PowerShell, CMD, and VS Code with live resize and crisp 256-color TUI rendering.
- **Positive:** Users **never** need to disable Windows Defender or SmartScreen to install or run `sshm`.
- **Positive:** Local privilege separation on multi-user Windows systems is enforced at the NTFS DACL layer.
- **Negative:** Requires maintaining build-tagged `*_windows.go` / `*_other.go` pairs and regenerating `.syso` PE metadata via `go generate ./cmd/sshm` on release builds.
