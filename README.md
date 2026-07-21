```
██████╗ ███████╗██╗   ██╗███████╗██╗   ██╗███╗   ██╗ ██████╗
██╔══██╗██╔════╝██║   ██║██╔════╝╚██╗ ██╔╝████╗  ██║██╔════╝
██║  ██║█████╗  ██║   ██║███████╗ ╚████╔╝ ██╔██╗ ██║██║
██║  ██║██╔══╝  ╚██╗ ██╔╝╚════██║  ╚██╔╝  ██║╚██╗██║██║
██████╔╝███████╗ ╚████╔╝ ███████║   ██║   ██║ ╚████║╚██████╗
╚═════╝ ╚══════╝  ╚═══╝  ╚══════╝   ╚═╝   ╚═╝  ╚═══╝ ╚═════╝
```

[![Go Version](https://img.shields.io/badge/go-1.26-blue)](go.mod)
[![Go Report Card](https://goreportcard.com/badge/github.com/Hennnnnnn/DevWorkspace)](https://goreportcard.com/report/github.com/Hennnnnnn/DevWorkspace)

End-to-end encrypted credential store. Push a secret from one device, pull it on another. The server sees only ciphertext — zero-knowledge.

**`devsync`** (no arguments) launches an interactive TUI with drill-down navigation. First run drops you into a setup wizard — no CLI commands or vault/team concepts required. `devsync <subcommand>` keeps the full CLI for scripts.

---

## Autocomplete (Tab)

```powershell
# One-time setup (auto-done by the installer)
devsync completion powershell | Out-String | Invoke-Expression

# Or install permanently:
devsync completion powershell >> $PROFILE
```

Press Tab to complete commands, flags, vault names, and team names.

---

## Install

### Windows (one-liner)

```powershell
irm https://raw.githubusercontent.com/Hennnnnnn/DevWorkspace/main/scripts/install.ps1 | iex
```

Installs `devsync` to `~\.devsync\bin`, adds to PATH, bakes in the default server URL. Ready to go.

### Go (any OS)

```sh
go install -ldflags '-X github.com/Hennnnnnn/DevWorkspace/internal/client/config.DefaultServerURL=https://devworkspace.onrender.com' github.com/Hennnnnnn/DevWorkspace/cmd/devsync@latest
```

---

## Quickstart: solo user

### TUI (recommended)

Run `devsync` with no arguments. First run launches a setup wizard: pick a username + passphrase, save the 24-word recovery phrase, done. The wizard runs `init` → `register` → `bootstrap-admin` → `unlock` and auto-creates a `personal` team + `main` vault behind the scenes — you land straight in the main menu ready to push/pull.

### CLI

```powershell
# 1. Generate a device keypair
devsync init

# 2. Register your device with the server
devsync register --username alice

# 3. Promote yourself to admin (solo user, first-time only)
devsync bootstrap-admin

# 4. Unlock your key into the agent
devsync unlock

# 5. Create a team + vault + push your first secret
devsync create-team eng
devsync create-vault secrets --team eng
devsync push .env --vault secrets
```

Run `devsync pull .env --vault secrets` to decrypt and download. `--vault` is optional on push/pull/history/checkout/rm — omitted, it auto-detects when you belong to exactly one vault.

---

## Interactive TUI

Run `devsync` with no arguments in a terminal. No keystore yet → the setup wizard. Otherwise, straight to the Bubble Tea TUI:

| View | Keys | What it does |
|------|------|-------------|
| **Wizard** (first run) | tab / enter | Username + passphrase → recovery phrase → auto register/unlock/team/vault |
| **Top menu** | enter | Select Vaults / Teams / Devices / Audit |
| **Vaults** | enter → files | Browse vaults, drill into file list (filtered to your active teams) |
| **Files** | p / u / d / enter | Pull, push (file picker), delete, version history |
| **History** | c | Checkout a specific version |
| **Teams** | enter → members, c / j / d | Joined / pending / not-joined sections; view members, create, join, delete |
| **Members** | a / p | Approve pending user (confirm fingerprint → pick vaults → grant in one flow), toggle pending filter |
| **Devices** | enter | Revoke a device (with confirm) |
| **Audit** | enter → vault | Read-only audit log per vault |

**Navigation**: `esc` goes back, `ctrl+c` quits. `U` unlocks the agent when locked. Destructive actions (delete, revoke) show a confirm dialog.

TUI calls the same `internal/client/actions` functions as the CLI — no behavior difference.

---

## Team workflow: share a secret

### Admin (you)

```powershell
# 1. Create a team (once)
devsync create-team eng

# 2. Create a vault (once)
devsync create-vault secrets --team eng

# 3. Push your secret
devsync push .env --vault secrets

# 4. Approve a teammate — verify fingerprint out-of-band first
devsync approve budi --fingerprint SHA256:xxxx   # fp from teammate's devsync init
# re-shares every vault key the admin holds to budi's device automatically

# Or in the TUI: Teams → members → a on a pending user →
# confirm fingerprint → pick which vaults to grant → done in one step
```

### Teammate (budi)

```powershell
devsync init
devsync register --username budi
# ^ send your fingerprint to the admin

devsync unlock
devsync pull .env --vault secrets
```

Budi's device gets the vault key sealed to it during `approve` (or a manual `grant`). He can now decrypt the file.

---

## Commands

| Category | Command | What it does |
|----------|---------|-------------|
| Setup | `init` | Generate device keypair (shows fingerprint) |
| | `register` | Register your public key with server |
| | `whoami` | Show your identity + status |
| | `unlock` / `lock` | Unlock key into agent for a period |
| | `bootstrap-admin` | Promote yourself to admin (first user) |
| | `config set` / `get` | View or change client config |
| Teams | `create-team` | Create a team (admin) |
| | `join` | Request to join a team (pending until approved) |
| | `teams` | List teams: joined / pending / not-joined |
| | `members` | List team members |
| Vaults | `create-vault` | Create a vault (admin) |
| | `grant` | Give vault access to someone (admin) |
| | `revoke` | Remove vault access + rotate key (admin) |
| | `approve` | Activate a pending user + re-share vault keys to their device (admin) |
| Files | `push` | Encrypt + upload a file (`--vault` optional, auto-detected) |
| | `pull` | Download + decrypt a file (`--vault` optional) |
| | `history` | Show file version history (`--vault` optional) |
| | `checkout` | Restore a specific version (`--vault` optional) |
| | `rm` | Soft-delete a file (`--vault` optional) |
| | `audit` | Show vault audit log |
| Devices | `device list` | List your devices |
| | `device add` | Authorize a new device |
| | `device revoke` | Revoke a device |
| | `devices of <user>` | Show another user's devices (admin) |

Run `devsync <command> --help` for details.

---

## Security model

- **Signed requests** — every API call carries an Ed25519 signature over `METHOD\npath\nauth-body-sha256\ntimestamp`. Server verifies against the stored device public key; timestamps outside ±5 minutes are rejected (anti-replay).
- **Zero-knowledge server** — vault data is encrypted with a symmetric key (X25519 + secretbox) before upload. The server only ever stores ciphertext and sealed key shares. It cannot read secrets.
- **Device-bound keys** — private key encrypted at rest with an Argon2id-derived key. Unlocked into an in-memory agent for a configurable TTL.
- **Per-vault key sealing** — vault keys are sealed to each device's X25519 box key. Revoking a user rotates the vault key and re-encrypts every file.

---

## Deployment

### Server env vars

| Variable | Default | Description |
|----------|---------|-------------|
| `DEVSYNC_DATABASE_URL` | — | Postgres DSN |
| `DEVSYNC_LISTEN_ADDR` | `:8080` | HTTP listen address |
| `PORT` | (fallback) | Alternative listen port (PaaS convention) |

```sh
docker compose up -d
# or standalone:
DEVSYNC_DATABASE_URL=postgres://... devsync-server serve
```

### Client

State lives in `~/.devsync/`. No config needed — server URL is baked into the binary.

---

## Project layout

```
cmd/devsync/             CLI entrypoint
cmd/devsync-server/      Server entrypoint
internal/
  client/
    commands/            Cobra CLI commands (thin wrappers calling actions)
    actions/             Business logic shared between CLI and TUI
    tui/                 Bubble Tea TUI (app.go, wizard, menu, header, vaults, teams, devices, audit, files, filepicker, confirm, unlock, theme)
    ...                  Config, keystore, agent, signed API client
  server/                HTTP handlers, auth middleware, Postgres store
  crypto/                E2E primitives (Ed25519, X25519, secretbox, Argon2id)
  protocol/              Wire contract (signing, headers, DTOs)
  db/                    Goose migrations + runner
```

---

## Development

```sh
make build           # -> bin/devsync(.exe), bin/devsync-server(.exe)
make test            # unit tests (no DB)
make up              # docker compose: postgres + server

# integration:
DEVSYNC_TEST_DATABASE_URL=postgres://devsync:devsync@localhost:5433/devsync?sslmode=disable \
  go test ./internal/server/... -run TestFullLifecycle
```

---

## License

MIT
