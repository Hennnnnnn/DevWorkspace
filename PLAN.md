# devsync — Implementation Plan

E2E credential store, akses gaya git/SSH. `push` file secret, `pull` di device lain. Multi-tim.

## Konsep

Bungkus workflow git/SSH untuk credential terenkripsi end-to-end. Server buta (zero-knowledge) — cuma simpan blob terenkripsi. Enkripsi/dekripsi selalu di client.

## Model Data

```
User ──has many──> Device (tiap device 1 keypair)
User ──member of──> Team
Team ──has many──> Vault
Vault ──granted to──> User (per-vault access)
Vault ──has many──> File (blob utuh, versioned)
```

- File = blob utuh (`.env`, cert, `id_rsa`, JSON). Server tak parse isi.
- Akses **per-vault** via grant (bukan otomatis semua anggota tim).

## Keamanan (E2E)

- Enkripsi client-side; server simpan blob terenkripsi + metadata.
- **Vault key** simetris **per-vault**, di-enkripsi ke public key tiap **device** anggota yang di-grant.
- Keypair **per-device** + passphrase (gaya SSH). Private key di `~/.devsync/`.
- Auth request = **signature** pakai device private key. Sign `(method + path + body-hash + timestamp)`. Server verifikasi pakai stored public key. Timestamp basi ditolak (anti-replay). Nol kredensial tambahan.
- **Agent** unlock gaya `ssh-agent`: `devsync unlock` sekali, key di memori agent + timeout konfigurable.

## Identitas & Onboarding

- **Self-register**: `devsync register` → generate keypair lokal → kirim `(username, device public key)` → status pending.
- **Join**: `devsync join <team>` → masuk antrian approval.
- **Approve (wajib fingerprint)**: admin `devsync approve <user> --fingerprint <fp>`. Fingerprint dikonfirmasi out-of-band. Kalau fingerprint tak cocok → ditolak. Saat approve, client admin enkripsi vault key ke device user (untuk vault yang di-grant).
- **Device pertama** user: admin approve (fingerprint). **Device berikut**: self-approve via device lama (device trusted tanda tangani device baru, gaya WhatsApp/Signal link-device).
- **Bootstrap admin pertama**: CLI di server (`devsync-server create-admin <user>`). Butuh shell access. Nol window balapan.

## Peran

- **Admin**: approve user, bikin team/vault, grant/revoke akses vault. Untuk grant, admin harus punya akses vault itu (megang key) — konsisten E2E.
- **Member**: push + pull vault yang di-grant.
- (Peran lebih halus / reader-only = enhancement nanti.)

## Konsistensi & Data

- **Version-check** per file (optimistic lock). Push stale ditolak → "pull dulu". Gaya git rejected push.
- **Soft delete** — file ditandai deleted, versi lama tetap di history. Bisa restore. `purge` = enhancement.
- **History + rollback**: `history <file>`, `checkout <file> --version N`.
- **Limit** ~1MB/file, jenis bebas.
- **Audit log** metadata: `(user, device, aksi, target, timestamp)`. Isi tak dicatat (tetap E2E). `devsync audit <vault>`.

## Revoke & Recovery

- **Revoke** per-device (device hilang) / per-vault (cabut akses).
- Setelah revoke → **rotate vault key** (generate key baru, re-encrypt semua file, distribusi ke device tersisa) + **reminder rotate secret asli** (pakai audit log untuk sebut file mana yang pernah diakses).
- **No backdoor**. Recovery via **re-share antar-anggota** (multi-holder: anggota lain re-encrypt vault key ke keypair baru user setelah register+approve ulang).
- `init` **paksa backup private key** (tampilkan, suruh simpan aman).

## Stack & Deploy

- Client + server **Go**, single static binary.
- **Postgres** (blob as BLOB/bytea, metadata relasional).
- Transport **REST over HTTPS + JSON**. Blob base64.
- Crypto: keypair + symmetric (arah age/NaCl-style — libsodium/`golang.org/x/crypto/nacl` atau `filippo.io/age`).
- Deploy: **self-hosted single instance**, docker-compose (server Go + Postgres), TLS via Caddy/nginx.
- Client `devsync config set server_url https://...`.

## Command CLI (lengkap dari awal)

Setup/identitas:
- `devsync init` — generate keypair+passphrase, paksa backup private key
- `devsync register` — kirim public key device ke server
- `devsync config set server_url <url>`
- `devsync unlock` — start agent session
- `devsync whoami`

Team/vault:
- `devsync create-team <name>` (admin)
- `devsync join <team>`
- `devsync teams`
- `devsync members` / `devsync members --pending`
- `devsync approve <user> --fingerprint <fp>` (admin)
- `devsync create-vault <name>` (admin)
- `devsync grant <user> <vault>` / `devsync revoke <user> <vault>` (admin)

File:
- `devsync push <file> --vault <v>`
- `devsync pull [<file>] --vault <v>`
- `devsync history <file> --vault <v>`
- `devsync checkout <file> --version N --vault <v>`
- `devsync rm <file> --vault <v>` (soft delete)
- `devsync audit <vault>`

Device:
- `devsync device add` — self-approve via device lama
- `devsync device list`
- `devsync device revoke <device>`

Server:
- `devsync-server create-admin <user>`

## Fase Build (urutan usul)

1. **Fondasi**: struktur repo Go (client + server), config server_url, Postgres schema + migrations, docker-compose.
2. **Crypto core**: keygen device (`init`), passphrase encrypt private key, backup flow, sign/verify request. Unit test isolasi.
3. **Auth transport**: middleware verifikasi signature di server, stored public key, anti-replay timestamp.
4. **Register + bootstrap**: `register`, `create-admin` (server CLI), user pending state.
5. **Team + approve**: `create-team`, `join`, `members --pending`, `approve --fingerprint`.
6. **Vault + grant**: `create-vault`, `grant`/`revoke`, vault key generation + enkripsi ke device keys.
7. **Push/pull**: enkripsi file client-side, upload/download blob, version-check, size limit.
8. **History/rollback/soft-delete**: `history`, `checkout`, `rm`.
9. **Multi-device**: `device add` (self-approve via device lama), `device list/revoke`.
10. **Revoke + rotate**: rotate vault key, re-encrypt, reminder rotate secret asli.
11. **Agent**: `unlock`, key di memori + timeout.
12. **Audit**: log tiap aksi, `audit <vault>`.
13. **Packaging**: single binary release, scoop/brew manifest.

## Threat Model (ringkas)

- Server bocor → attacker cuma dapat blob terenkripsi (E2E).
- Key substitution di register → ditutup fingerprint verification wajib saat approve.
- Anggota keluar → revoke + rotate + rotate secret asli (audit sebut file terdampak).
- Device hilang → revoke per-device tanpa ganggu device lain.
- Replay request → timestamp + verifikasi signature.
- Lupa key → recovery via multi-holder re-share; no backdoor.
