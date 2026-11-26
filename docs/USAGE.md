# sshm — Complete User Guide (Windows)

> **Publisher:** Oktanetflow · **Copyright © 2026 Oktanetflow**

## 1. Installation & Terminal Setup

### Via Windows EXE Installer (Recommended)
Run `sshm-setup-1.0.0-amd64.exe`. It installs per-user into `%LOCALAPPDATA%\Programs\Oktanetflow\sshm` (no admin UAC prompt needed) and registers `sshm` in your Windows User `PATH`. Reopen Windows Terminal or PowerShell after installing.

### Optional PowerShell Tab Completion
Add to your PowerShell `$PROFILE`:
```powershell
sshm completion powershell | Out-String | Invoke-Expression
```

---

## 2. Interactive Agile TUI (`sshm`)

Launch with no arguments:
```powershell
sshm
```

| Key | Action |
| :--- | :--- |
| `↑` / `↓` or `j` / `k` | Navigate hosts and groups |
| `Enter` | Connect to selected host (or expand/collapse group) |
| `/` | Fuzzy search across alias, hostname, user, group, jump host, tags, and notes |
| `Tab` / `Shift+Tab` | Cycle group filter tabs (`All`, `prod`, `staging`, …) |
| `Space` / `←` / `→` | Fold / unfold selected group |
| `i` | Toggle **Split-Pane Host / Group Inspector** (auto-enabled on terminals ≥ 100 columns) |
| `r` | Refresh **live async TCP reachability & latency** probes (`● 14ms` / `● off`) |
| `t` | Cycle UI theme live (`minimal-dark` → `minimal-light` → `mono`) & save to `config.toml` |
| `a` | Add new host (Save Password enabled by default; includes Tags & Notes) |
| `e` | Edit selected host (preserves tags, notes, pin, tunnels, and credentials) |
| `f` | Open **Interactive Port Forward Manager** (`-L`, `-R`, `-D`) for selected host |
| `d` | Delete selected host and its stored password / key passphrase |
| `p` | Pin / unpin selected host to top of its group |
| `g` | Create or edit a group & inherited defaults (color, user, port, key, jump) |
| `?` / `q` | Help modal / Quit |

**Inside Add / Edit Host Form:** `Tab` / `↑↓` moves between fields, `Space` / `←→` toggles Auth Type or Save Password, `Enter` saves, `Ctrl+S` saves & connects immediately, `Esc` cancels.

**Inside Port Forward Manager (`f`):** Press `a` to add a new tunnel, `Tab`/`←→` to choose `local` / `remote` / `dynamic`, enter spec (e.g. `5432:127.0.0.1:5432` or `1080`), press `Enter` to save; press `d` to remove a selected tunnel.

---

## 3. Daily CLI Commands

```powershell
# Connect directly by alias
sshm prod-api

# Ad-hoc connect (IPv4 or bracketed IPv6; prompts to save host + password)
sshm root@192.168.1.50
sshm ubuntu@[2001:db8::1]:2222

# List hosts (grouped, flat, or JSON)
sshm ls
sshm ls -g prod
sshm ls --flat --json

# Add a host (masked password saved to Windows Credential Manager by default)
sshm add prod-api --host 10.0.1.12 --user root --group prod --tags api,linux
sshm add prod-key --host 10.0.1.13 --user ubuntu --key ~/.ssh/id_ed25519
sshm add temp-box --host 10.0.9.9 --user admin --no-save-password

# Edit or remove a host (also deletes saved password & key passphrase)
sshm edit prod-api
sshm rm prod-api

# Print version & Oktanetflow copyright
sshm --version
```

---

## 4. Groups & Inherited Defaults

```powershell
sshm group create prod --color red --default-user deploy --default-port 2222 --default-key ~/.ssh/id_ed25519 --default-jump bastion
sshm group ls
sshm group rename staging stg
sshm group rm stg                    # Moves hosts to 'default'
sshm group rm stg --delete-hosts     # Removes hosts and all saved credentials
```

---

## 5. Power Features

```powershell
# Run a command on one host or an entire @group concurrently
sshm exec prod-api "uptime"
sshm exec @prod "df -h" --parallel

# Ad-hoc port forwarding: Local (-L), Remote (-R), Dynamic SOCKS5 (-D)
sshm prod-db -L 5432:127.0.0.1:5432
sshm prod-api -L [::1]:8080:127.0.0.1:80
sshm prod-api -D 1080

# Copy files over SFTP
sshm cp .\app.zip prod-api:/opt/app/
sshm cp prod-api:/var/log/app.log .\app.log

# Import existing configs
sshm import --ssh-config
sshm import --putty

# Encrypted backup / restore (Argon2id + AES-256-GCM)
sshm export backup.sshm.enc
sshm import --backup backup.sshm.enc
```

---

## 6. Configuration & Security

- **Config Location:** `%APPDATA%\sshm\config.toml` and `%APPDATA%\sshm\hosts.toml` (protected with owner-only Windows NTFS DACLs; inspect paths via `sshm config path`).
- **Validated Settings:**
  ```powershell
  sshm config set credential_backend wincred          # wincred | dpapi | vault
  sshm config set save_password_by_default true       # true | false
  sshm config set theme minimal-dark                  # minimal-dark | minimal-light | mono
  sshm config set auto_probe_on_start true            # true | false
  sshm config set strict_host_key_checking ask        # ask | yes | no
  sshm config set keepalive_interval_sec 30           # >= 0
  ```
- **Host Key Changed Warning:** Verify the server rebuild, then remove the old key with:
  ```powershell
  sshm hostkey remove <alias>
  ```
