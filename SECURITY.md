# Security Policy — sshm by Oktanetflow

> **Copyright © 2026 Oktanetflow**

## Security Architecture

`sshm` is engineered to keep SSH credentials and host metadata secure on Windows without requiring third-party wrappers or disabling OS protections:

| Security Control | Implementation |
| :--- | :--- |
| **Zero Plaintext Passwords on Disk** | `hosts.toml` stores only `credential_ref` identifiers (`sshm:<id>`). Passwords and key passphrases live exclusively in Windows Credential Manager (`CredWriteW`/`CredReadW`), Windows DPAPI (`CryptProtectData`), or an `Argon2id` + `AES-256-GCM` encrypted vault (`vault.enc`). |
| **Windows NTFS Owner-Only ACLs** | `%APPDATA%\sshm\` is hardened via `SetNamedSecurityInfo` (`PROTECTED_DACL_SECURITY_INFORMATION`) to restrict access strictly to the current Windows user SID and `NT AUTHORITY\SYSTEM`. |
| **Memory Hygiene** | Password slices, DPAPI decrypted blobs (prior to `LocalFree`), exported JSON buffers, and cached vault passphrases (`VaultStore.Close`) are explicitly zeroed in memory. |
| **MITM Host Key Verification** | Verifies server keys against OpenSSH-compatible `~/.ssh/known_hosts` with TOFU prompts and hard aborts on key mismatch. |
| **Atomic Persistence** | Config, hosts, DPAPI store, vault, and `known_hosts` updates write to unique temporary files with `fsync` before atomic rename. |
| **Network Hardening** | Dynamic SOCKS5 proxies enforce a 10-second handshake deadline and emit a loud warning if bound to a non-loopback interface. |
| **Least-Privilege Execution** | Embedded Win32 manifest declares `requestedExecutionLevel level="asInvoker"` — `sshm` and its per-user installer never request UAC Administrator elevation. |
| **Zero Packing / Reproducible Builds** | Compiled with `CGO_ENABLED=0` and `-trimpath`. **UPX and obfuscators are strictly forbidden.** |

---

## Verifying Releases with Windows Security Enabled

**Never disable Windows Defender or SmartScreen to run `sshm`.**

1. **Check SHA-256 Checksum:**
   ```powershell
   Get-FileHash .\sshm-setup-1.0.0-amd64.exe -Algorithm SHA256
   ```
   Compare the output against `SHA256SUMS.txt` attached to the official GitHub Release.
2. **Check Embedded Publisher Metadata & Authenticode Signature:**
   ```powershell
   (Get-Item .\sshm.exe).VersionInfo | Format-List CompanyName, ProductName, ProductVersion, LegalCopyright
   Get-AuthenticodeSignature .\sshm.exe
   ```
3. **GitHub Artifact Attestation Verification:**
   ```powershell
   gh attestation verify sshm-setup-1.0.0-amd64.exe --owner Oktanetflow
   ```

If you are maintaining releases for Oktanetflow and want zero SmartScreen warnings for first-time downloaders, follow [docs/WINDOWS_PUBLISHER_GUIDE.md](docs/WINDOWS_PUBLISHER_GUIDE.md).

---

## Automated Security Gates

Every commit and release runs:
- `go vet ./...`
- `go test -race ./...`
- `govulncheck ./...` (Go Vulnerability Database CVE audit)
- `gosec -severity medium ./...` (Go static security analyzer)

Run all gates locally:
```powershell
powershell -ExecutionPolicy Bypass -File scripts/security-scan.ps1
```

---

## Reporting a Vulnerability

Please **do not** open public GitHub issues for security vulnerabilities.
Report privately via GitHub Security Advisories at `https://github.com/Oktanetflow/sshm/security/advisories/new` (or contact Oktanetflow security maintainers directly). We aim to acknowledge reports within 48 hours.
