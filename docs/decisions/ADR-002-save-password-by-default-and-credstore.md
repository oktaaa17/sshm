# ADR-002: Save-Password-by-Default with Three-Tier Credential Store

## Status
Accepted

## Date
2026-11-26

## Context
Many Windows infrastructure engineers, network operators, and lab users connect to hosts that rely on password authentication or passphrase-protected SSH keys.
- Asking users to re-type passwords on every reconnect creates friction and drives risky habits (saving plaintext passwords in scripts, sticky notes, or `.bat` files).
- Conversely, storing plaintext passwords in `hosts.toml` is an unacceptable security vulnerability (risk of accidental Git commits, backup leaks, or disk snooping).
- Different Windows deployment scenarios require different secret storage models: standard interactive Windows desktops benefit from Windows Credential Manager, headless/restricted sessions may prefer DPAPI, and portable USB / cross-platform users need a self-contained encrypted file vault.

## Decision
1. Enable **Save Password by Default** (`general.save_password_by_default = true`) while guaranteeing **zero plaintext secrets on disk** (`hosts.toml` stores only opaque `credential_ref` identifiers like `sshm:<host-id>`).
2. Abstract secret persistence behind `internal/credstore.Store` with three pluggable backends:
   - **`wincred` (Default on Windows):** Native Windows Credential Manager via `advapi32.dll` (`CredWriteW` / `CredReadW` / `CredDeleteW`).
   - **`dpapi`:** Windows DPAPI (`crypt32.dll` `CryptProtectData` / `CryptUnprotectData`) bound to the current Windows user account, persisted atomically.
   - **`vault`:** Cross-platform portable encrypted vault (`vault.enc`) using **Argon2id** key derivation + **AES-256-GCM** authenticated encryption.
3. Enforce strict **memory hygiene**: zero password slices, DPAPI decrypted `outBlob` buffers prior to `LocalFree`, and cached vault passphrases via `VaultStore.Close()`.
4. Ensure host deletion (`sshm rm`) and group host deletion (`sshm group rm --delete-hosts`) delete both host passwords and key passphrases (`credstore.KeyPassphraseRef(h.ID)`), and `UpsertHost` preserves existing `ID`/`CredentialRef` on alias update so secrets are never orphaned.

## Alternatives Considered

### Never save passwords (SSH keys only)
- **Pros:** Simplest implementation; zero secret-store maintenance.
- **Cons:** Poor UX for mixed Windows/Linux ops teams, appliances, jump boxes, and freshly provisioned VMs where passwords are required initially or permanently.
- **Rejected:** Directly undermines `sshm`'s value proposition as a frictionless Windows SSH manager.

### Store Base64 or obfuscated passwords in `hosts.toml`
- **Pros:** Easy to implement and portable.
- **Cons:** Base64 is not encryption; any backup or accidental sync exposes raw credentials immediately.
- **Rejected:** Violates fundamental security engineering principles.

### Single OS keyring library (external third-party wrapper)
- **Pros:** Less platform-specific Win32 code.
- **Cons:** Many Go keyring libraries pull CGo dependencies or lack Windows DPAPI / Argon2id fallback and explicit memory-zeroing guarantees.
- **Rejected:** Direct `golang.org/x/sys/windows` Win32 calls keep `CGO_ENABLED=0` intact with full control over buffer zeroing and atomic file writes.

## Consequences
- **Positive:** Users get one-keystroke reconnects out of the box without ever writing a plaintext password to disk.
- **Positive:** Users can opt out globally (`sshm config set save_password_by_default false`) or per host (`--no-save-password` / TUI toggle).
- **Positive:** Encrypted export/import (`sshm export backup.sshm.enc`) enables safe workstation migration using Argon2id + AES-256-GCM.
- **Negative:** Running `wincred` inside non-interactive Windows CI runners without a logged-in desktop logon session requires graceful handling (addressed by DPAPI/Vault fallback in tests).
