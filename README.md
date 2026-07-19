# devsync

[![Go Version](https://img.shields.io/badge/go-1.26-blue)](go.mod)
[![Go Report Card](https://goreportcard.com/badge/github.com/Hennnnnnn/DevWorkspace)](https://goreportcard.com/report/github.com/Hennnnnnn/DevWorkspace)

End-to-end encrypted credential store with git/SSH-style access. Push a secret from one device, pull it on another — the server is zero-knowledge.

---

## Install

### Windows (one-liner)

```powershell
irm https://raw.githubusercontent.com/Hennnnnnn/DevWorkspace/main/scripts/install.ps1 | iex
```

Installs `devsync` + `devsync-server` to `~\.devsync\bin`, adds to PATH, and bakes in the default server URL. Ready to use — no config needed.

### Go (any OS)

```sh
go install -ldflags '-X github.com/Hennnnnnn/DevWorkspace/internal/client/config.DefaultServerURL=https://devworkspace.onrender.com' github.com/Hennnnnnn/DevWorkspace/cmd/devsync@latest
```

### From source

```sh
git clone https://github.com/Hennnnnnn/DevWorkspace.git
cd DevWorkspace
.\scripts\install.ps1 -Build      # Windows
# or
make build DEFAULT_SERVER_URL=https://devworkspace.onrender.com   # Linux/macOS
```

---

## Quickstart

```sh
cp .env.example .env
make up

devsync init
devsync register --username alice
# on the server host:
docker compose exec server devsync-server create-admin alice <fingerprint>

devsync unlock
devsync create-team eng
devsync create-vault secrets --team eng
devsync push .env --vault secrets
devsync pull .env --vault secrets
```

> **Self-host tip:** build with `make build DEFAULT_SERVER_URL=https://your-server.com` and users won't need `config set server_url`.

---

## Security model

- **Signed requests** — every API call carries an Ed25519 signature over `METHOD\npath\nauth-body-sha256\ntimestamp`. Server verifies against the stored device public key; timestamps outside ±5 minutes are rejected (anti-replay).
- **Zero-knowledge server** — vault data is encrypted with a symmetric key (X25519 + secretbox) before upload. The server only ever sees ciphertext and sealed key shares. It cannot read secrets.
- **Device-bound keys** — private key encrypted at rest with an Argon2id-derived key. Unlocked into an in-memory agent for a configurable TTL.
- **Per-vault key sealing** — vault keys are sealed to each device's X25519 box key. Revoking a user rotates the vault key and re-encrypts every file.

---

## Commands

| Command | Description |
|---------|-------------|
| `init` | Generate a device keypair |
| `register` | Register device public key with server |
| `whoami` | Show current identity |
| `unlock` / `lock` | Unlock device key into agent |
| `config set` / `get` | Client configuration |
| `create-team` | Create a team |
| `join` | Request to join a team |
| `teams` | List teams |
| `members` | List team members |
| `approve` | Approve a pending user |
| `create-vault` | Create a vault |
| `grant` | Grant vault access |
| `revoke` | Revoke vault access + rotate key |
| `push` | Encrypt and upload a file |
| `pull` | Download and decrypt a file |
| `history` | Show file version history |
| `checkout` | Restore a specific version |
| `rm` | Soft-delete a file |
| `audit` | Show vault audit log |
| `device list` / `add` / `revoke` | Manage devices |

Run `devsync <command> --help` for full usage and argument details.

---

## Deployment

### Server

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | `postgres://devsync:devsync@db:5432/devsync` | Postgres DSN |
| `PORT` | `8080` | HTTP listen port |
| `HMAC_SECRET` | auto-generated | Request signing HMAC key |

```sh
docker compose up -d
# or standalone:
DATABASE_URL=postgres://... ./bin/devsync-server serve
```

### Client

```sh
# per-project or system-wide
devsync config set server_url https://devsync.example.com
```

State lives in `~/.devsync/`.

---

## Project layout

```
cmd/devsync/             CLI entrypoint
cmd/devsync-server/      Server entrypoint
internal/
  client/                Config, keystore, agent, signed API client, Cobra commands
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
