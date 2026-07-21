# Security Model — devsync

devsync encrypts secrets end-to-end so a self-hosted server never sees plaintext. This document defines what that promise means, what it does not cover, and where the attacker sits.

---

## What is protected

| Asset | Protection |
|-------|-----------|
| **Secret file contents** (vault blobs) | Encrypted client-side with NaCl secretbox (XSalsa20-Poly1305). Server stores ciphertext only. |
| **Vault encryption keys** | Sealed to each authorized device's X25519 box key via `box.SealAnonymous` (ephemeral sender, anonymous). Server relays sealed keys but cannot open them. |
| **Private key at rest on client** | Encrypted with Argon2id-derived key + NaCl secretbox. Stored as `device.key` with `0600` permissions. |
| **Recovery phrase** | 24-word BIP39 mnemonic printed once during `init`. Never transmitted. 32-byte seed derives Ed25519 + X25519 keys via HKDF-SHA256. |
| **API request integrity** | Every request signed with Ed25519. Anti-replay via ±5 minute timestamp window. |

## What is NOT protected (visible to the server)

The server is the **trusted custodian of metadata**. Zero-knowledge covers content, not metadata:

| What the server can see | Detail |
|--------------------------|--------|
| Usernames | Plaintext in `users` table + auth headers |
| Device names and fingerprints | Plaintext in `devices` table |
| Team names | `teams` table |
| Vault names | `vaults` table |
| File paths (filenames) | `files` table — names are not encrypted |
| File sizes (ciphertext length) | `file_versions.size_bytes` |
| File version history | Version numbers, timestamps, author device fingerprint |
| Access graph | Who is in which team, who has which vault key share, which device pushed which version |
| Audit log | Username, device, action, target, timestamp — all plaintext for operator visibility |
| Invite tokens | Token value, target username, expiry |
| Online status / request patterns | IPs, request timing, frequency (no traffic analysis countermeasure) |

**If metadata sensitivity is a concern, do not trust the server with it.** Defense is trust-the-server for availability + trust-the-crypto for confidentiality.

## Cryptographic primitives

| Primitive | Library | Use |
|-----------|---------|-----|
| Ed25519 | `crypto/ed25519` | Device identity, request signing |
| X25519 | `go.dedis.ch/kyber` or `crypto/ed25519` (derived) | Box key exchange |
| NaCl Secretbox (XSalsa20-Poly1305) | `golang.org/x/crypto/nacl/secretbox` | Vault blob encryption, private key encryption at rest |
| NaCl Box (anonymous seal) | `golang.org/x/crypto/nacl/box` | Vault key sharing |
| Argon2id | `golang.org/x/crypto/argon2` | Passphrase-based key derivation (time=3, memory=64 MiB, threads=4) |
| SHA-256 | `crypto/sha256` | Fingerprints, body hashing, BIP39 checksum |
| HKDF-SHA256 | `golang.org/x/crypto/hkdf` | Recovery seed → key pair derivation |
| BIP39 | Embedded wordlist | Recovery phrase encoding (24 words) |
| `crypto/rand` | Go stdlib | All nonce, seed, and key generation |

## Attack surface

### Server-side

| Attack vector | Mitigation |
|---------------|-----------|
| **Replay attacks** | 5-minute timestamp window (`protocol.MaxSkewSeconds = 300`) |
| **Signature forgery** | Ed25519 signature verified server-side on every authed request; fingerprint verified on register |
| **Unauthorized access** | Three-tier auth: `authed` (signature valid) → `activeAuthed` (device+user active) → `adminAuthed` (admin flag) |
| **Brute-force bootstrap** | One-shot: `/admin/bootstrap` only works when zero admins exist, and caller must know the fingerprint of a registered device |
| **Brute-force registration** | Rate-limited (token bucket: 20 burst, 2/sec refill per IP) on `/register` and `/admin/bootstrap` |
| **Token replay** | Invite tokens are single-use, atomically claimed in a DB transaction, 24h expiry |
| **SQL injection** | Parameterized queries via the Store abstraction |
| **Fingerprint spoofing** | Server verifies `crypto.Fingerprint(signPub) == req.Fingerprint` on register |
| **Body size DoS** | `io.LimitReader` (2 MiB) on authed bodies; `http.MaxBytesReader` (1 MiB) on registration |
| **Request body reads** | Body read once and stored in context; no double-read risk |
| **Server compromise** | Attacker with DB access can read all metadata but cannot decrypt files or vault keys. Can modify ciphertext (integrity detection on client decrypt). Can delete data (no server-side backup). Can approve malicious devices as admin. |

### Client-side

| Attack vector | Mitigation |
|---------------|-----------|
| **Disk compromise** | Private key at rest encrypted via Argon2id + secretbox |
| **Passphrase brute-force** | Argon2id (memory-hard). Weak passphrases are the user's responsibility. |
| **Memory extraction** | Keys exist in process memory after unlock. No `mlock` or `madvise(MADV_DONTDUMP)` — standard Go process |
| **Recovery phrase leak** | Printed once. User must store offline. Anyone with the phrase can recover keys unconditionally (no passphrase needed). |
| **Keystore file tampering** | Atomic write via temp-file + rename |
| **Supply chain** | Go module deps: `golang.org/x/crypto`, `cobra`, `bubbletea`. No CGo. Binary builds are reproducible with Go toolchain. |

### Network

| Attack vector | Mitigation |
|---------------|-----------|
| **TLS missing** | Server does not enforce TLS. Server operator must place it behind a reverse proxy with TLS (nginx, Caddy). Client connects to whatever URL configured. |
| **MITM (no TLS)** | Attacker can replay, modify, or drop ciphertext. Cannot decrypt without private keys. Can DoS. |

## What the server CANNOT do

- Decrypt file contents or vault keys (does not hold any private key material)
- Impersonate a device (cannot sign without the private key)
- Derive recovery keys from mnemonic (recovery is client-side only)
- Forge signatures that pass client verification

## What the server CAN do (and you trust it not to)

- Reject or delay requests (availability)
- Modify or delete stored ciphertext (integrity)
- Approve a malicious device as admin (once admin, they control team/vault access)
- Log all metadata permanently (privacy)
- Serve stale or wrong data (integrity)

## Threat model for the self-hosted operator

**Adversary:** server operator (or anyone who compromises the server) wants to read your secrets.

**Result:** they cannot. Contents and vault keys are encrypted client-side. They see metadata only.

**Adversary:** server operator wants to impersonate you.

**Result:** they cannot without your private key.

**Adversary:** server operator wants to prevent you from accessing your data.

**Result:** they can (availability). Mitigate with backups of the DB + ciphertext.

**Adversary:** someone steals your `device.key` file plus your passphrase.

**Result:** full compromise. Mitigate with a strong passphrase and disk encryption.

**Adversary:** someone gets your 24-word recovery phrase.

**Result:** full compromise. The phrase is the master key — store it like a hardware wallet seed.

## Known gaps (not implemented)

| Gap | Severity | Plan |
|-----|----------|------|
| No TLS enforcement at app level | Medium | Documented: use reverse proxy. May add flag later. |
| No 2FA | Medium | Not planned. Device key is the second factor (something you have). |
| No passphrase strength check beyond 8 chars | Low | User responsibility. |
| No memory locking (`mlock`) | Low | Standard Go limitation. Add if server-side threat grows. |
| No encrypted backup of recovery phrase on server | Low | Deliberately skipped. BIP39 mnemonic is the backup. |
| No rate limiting on authed endpoints | Medium | Authed requests use signature (expensive to forge). Registration/bootstrap are rate-limited. |
| No WebAuthn/Hardware token support | Low | Future enhancement. |
| Metadata is fully visible to server | Medium | By design. Not zero-knowledge metadata. |
| Invite tokens are bearer tokens (no PK binding) | Low | Single-use, 24h expiry, atomically claimed. |
| No forward secrecy on vault keys | Low | Vault keys are static. Rotate via revoke+grant. |
| Server code is not audited | High | All crypto is stdlib / `x/crypto`. Threat model in this document. Tests in progress. |

## Reporting a vulnerability

If you discover a security issue, **do not open a public issue**. Contact the maintainer directly.

---

*This document describes devsync as of 2025-07-21. Last updated with commit 8f3a7b4.*
