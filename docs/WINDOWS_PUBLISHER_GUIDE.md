# Windows Publisher & SmartScreen Guide — Oktanetflow

> **Copyright © 2026 Oktanetflow**

This guide explains how Oktanetflow builds, signs, and distributes `sshm.exe` and `sshm-setup-<version>-amd64.exe` so users can run them **with Windows Defender and Windows SmartScreen fully enabled** — never needing to disable Windows Security.

---

## 1. Why Unsigned Windows Binaries Trigger SmartScreen

Microsoft Defender SmartScreen evaluates downloaded executables using **Authenticode digital signatures** and **reputation telemetry** — not by scanning for evasion tricks.

What we **already build into every release** out of the box:
1. **Valid Win32 PE `VersionInfo`** (`CompanyName: Oktanetflow`, `ProductName: sshm`, `LegalCopyright: Copyright © 2026 Oktanetflow`) embedded via `cmd/sshm/versioninfo.json`.
2. **Least-Privilege `asInvoker` Application Manifest** (`cmd/sshm/sshm.exe.manifest`) — declares standard-user execution (zero UAC admin prompt) and Windows 10/11 UTF-8 compatibility.
3. **Zero UPX / Zero Packers** — UPX-packed Go executables frequently trigger heuristic AV false positives; our build scripts strictly prohibit packing.
4. **Reproducible `-trimpath` Builds + `SHA256SUMS.txt` + GitHub Artifact Attestations** — enables cryptographic verification of every release artifact.
5. **Per-User Inno Setup Installer (`PrivilegesRequired=lowest`)** — installs cleanly to `%LOCALAPPDATA%\Programs\Oktanetflow\sshm` without administrator escalation.

---

## 2. Eliminating SmartScreen "Unknown Publisher" Warnings (Authenticode Signing)

To display **Verified publisher: Oktanetflow** with zero SmartScreen warning for every user on Windows 10/11, sign both `sshm.exe` and `sshm-setup-<version>-amd64.exe` using one of two trusted options:

### Option A: Microsoft Trusted Signing (Recommended — Instant SmartScreen Trust)
- Microsoft's cloud signing service (~$9.99/month for Verified Identity / Organization).
- Certificates issued by Microsoft Trusted Signing receive **immediate SmartScreen reputation** (no months of download reputation building).
- Integrates directly with GitHub Actions via `azure/trusted-signing-action`.

### Option B: Standard OV / EV Code Signing Certificate (`.pfx` or Cloud HSM)
1. Obtain a code signing certificate from a Microsoft-trusted CA (DigiCert, Sectigo, SSL.com, GlobalSign).
2. Add two encrypted **GitHub Repository Secrets** in `Oktanetflow/sshm → Settings → Secrets and variables → Actions`:
   - `AUTHENTICODE_PFX_BASE64`: Base64-encoded `.pfx` certificate file
   - `AUTHENTICODE_PFX_PASSWORD`: Certificate export password
3. Push a version tag (`git tag v1.0.0 && git push origin v1.0.0`) — `.github/workflows/release.yml` will automatically sign `sshm.exe`, `sshm-setup-1.0.0-amd64.exe`, and generate `SHA256SUMS.txt` with an RFC 3161 DigiCert timestamp!

### Local Signing Command
```powershell
.\packaging\windows\sign.ps1 -Files @("dist\sshm.exe", "dist\sshm-setup-1.0.0-amd64.exe") -PfxPath C:\certs\oktanetflow.pfx
```

---

## 3. Preventing Windows Defender False Positives (Microsoft WDSI)

For each public release (especially before signing or immediately after publishing a new version):
1. Submit `sshm-setup-<version>-amd64.exe` and `sshm-<version>-windows-amd64.exe` as a **Software Developer** to the **Microsoft Security Intelligence Portal**:
   - URL: https://www.microsoft.com/en-us/wdsi/filesubmission
2. Select **"Software developer"** → **"Incorrectly detected as malware/PUA"** (or proactive clean-file analysis).
3. Microsoft typically updates Defender cloud signatures globally within 24–48 hours.

---

## 4. Verifying Publisher Integrity Locally

```powershell
# Inspect embedded Oktanetflow PE metadata
(Get-Item .\dist\sshm.exe).VersionInfo | Format-List CompanyName, ProductName, ProductVersion, LegalCopyright

# Verify Authenticode signature (after signing)
Get-AuthenticodeSignature .\dist\sshm.exe | Format-List Status, StatusMessage, SignerCertificate, TimeStamperCertificate
```
