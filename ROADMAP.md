# devsync — Roadmap to Real Users

**Goal:** an open-source tool people actually run (primary), that doubles as a strong portfolio piece (bonus).

**Audience:** random developer on the internet who installs `devsync`, self-hosts the server, and trusts it with real secrets.

**Server model:** self-host first (single binary), hosted `devworkspace.onrender.com` as a try-it-now demo only.

> Note: `PLAN.md` covers the (completed) TUI build. This file is the next phase — the road to actual users.
>
> **Progress:** Items #1, #2, #3 shipped. Next: #4 (recovery code).

---

## Guiding constraints

- **Self-host ease is feature #1.** Every step of friction between "curl" and "running server" loses users.
- **Zero-knowledge must be credible, not just claimed.** Trust signals (tests, threat model) are product features here, not chores.
- **Solo single-device user is the majority path.** No design decision may strand them (recovery, onboarding).
- Windows-only installer is acceptable for now; cross-platform is a later enhancement.

---

## Work items (priority order — by impact/dependency)

### 1. Server single-binary (SQLite default, Postgres opt-in) ✅ DONE

**Why:** Postgres + Docker requirement is the biggest adoption funnel killer for "just let me try it." Also a clean architecture story for interviews (one interface, two backends).

- ✅ Extract `Store` into dialect-based struct (not interface — pragmatic: `rebind()` + `forUpdate()` handle 5% divergence).
- ✅ Implement SQLite backend: `modernc.org/sqlite` (pure Go, no CGo), `SetMaxOpenConns(1)`, pragmas (FK, WAL, busy_timeout).
- ✅ SQLite default: `devsync-server serve` → single binary, DB = `UserConfigDir/devsync/devsync.db`, no Docker/Postgres needed.
- ✅ Postgres opt-in via `DEVSYNC_DATABASE_URL` with `postgres://` prefix.
- ✅ Migrations split: `migrations/postgres/` + `migrations/sqlite/`, Goose handles both.
- ✅ Integration tests default to SQLite (`t.TempDir()`), Postgres opt-in via `DEVSYNC_TEST_DATABASE_URL`.

**Shipped in:** `592fdf5` (2025-07-20).

### 2. Fix solo onboarding (small) ✅ DONE

**Why:** `devsync setup` already exists and covers ~80% of solo onboarding, but step 3 only *prints* the `bootstrap-admin` instruction instead of running it (`internal/client/commands/setup.go` ~L76–83). The solo user has to drop out, run it manually, and re-enter — funnel leak.

- ✅ Detect first-user safely (guard so it no-ops if an admin already exists) — shipped in `25361d6`.
- ✅ Auto-run `bootstrap-admin` inside `setup` when the user is the first/only user.

**Pending UX polish:** `bootstrap-admin` and `unlock` (step 4) both prompt for passphrase separately — user types it twice. Refactor to share passphrase across steps if UX complaint arises.

### 3. Invite token flow (replaces manual fingerprint exchange) ✅ DONE

**Why:** sharing a secret today requires the admin and teammate to exchange an SSH-style fingerprint over an external channel — the most error-prone and security-sensitive step. Server already has `handleInvite`; needs a token layer.

- ✅ Admin: `devsync invite <user> --team <team>` → issues a single-use invite token (24h expiry, base32 encoded).
- ✅ Teammate: `devsync join <token>` → claims token, auto-activates user+device, adds to team as active member.
- ✅ Token binds expected username so it can't be replayed by a third party.
- ✅ `invite_tokens` table (SQLite + Postgres migrations), atomic claim in transaction.
- ✅ Backward-compatible: `devsync join <team>` still works for name-based join requests.

**Done when:** onboarding a teammate needs no manual fingerprint exchange.

### 4. Recovery code (mnemonic key recovery)

**Why:** today there is **no recovery, no backup, no passphrase change**. The private key lives only on the device, encrypted by the passphrase. Lost passphrase / lost device / dead disk = **all secrets gone permanently** (unless a second device is still linked). For solo single-device users (the majority) the risk is 100%. This is a silent adoption killer and generates bad reviews from the first user who loses data.

- On `init`, generate a recovery phrase (mnemonic / recovery-seed, hardware-wallet style).
- User stores it offline.
- New device can restore the private key from the recovery phrase without the old passphrase.
- Heaviest item on the list: derive key material from the recovery seed and store an encrypted backup share.

**Done when:** a user who lost their device/passphrase can recover access with only the recovery phrase.

### 5. Trust signals (order: B → A → C)

**Why:** this is a crypto tool with 3 test files for ~6,700 LOC, no audit, no written threat model. The zero-knowledge claim is unproven to the reader. Random devs (adoption) and recruiters (portfolio) both ask "why should I believe this is safe?"

**5B — `SECURITY.md` (threat model) — cheapest, do first.**
- What is protected, what is not.
- Attack surface.
- Explicitly: what the server *can* and *cannot* see — including **metadata** (usernames, team/vault/file names, timestamps, access graph) which is NOT hidden by zero-knowledge. Users deserve to know.

**5A — Test coverage for crypto / protocol / auth.**
- Raise coverage in `internal/crypto`, `internal/protocol`, and server auth middleware.
- Surface a coverage badge.

**5C — Adversarial / property tests.**
- Tests that try to *break* it: replay attack rejected, tampered signature rejected, wrong-key decryption fails, timestamp outside ±5 min rejected.
- Active proof, not just a coverage number.

**Done when:** the security posture is defensible in an interview and legible to a cautious new user.

---

## Alternate execution order (quick-win first)

If momentum matters more than strict impact order:

`2 ✅ → 1 ✅ → 3 → 5 → 4`

Items #1, #2, #3 shipped. Next: #4 (recovery code).

*(Ordering decision still open — see bottom.)*

---

## Explicitly out of scope for now — future enhancements

Recorded so they are not forgotten, deliberately deferred:

- **Hosted-at-scale** — the hosted demo is single-server, best-effort. No SLA, no scaling, no quotas. Do not build billing, multi-tenant limits, or uptime guarantees yet.
- **Billing / quotas / storage limits** — only relevant if hosted becomes a real service, which is not the current model.
- **Cross-platform installer** — macOS/Linux one-liner installers. Windows-only for now; `go install` covers other platforms.
- **Passphrase change / key rotation UX** — beyond recovery (#4), a first-class "change my passphrase" flow.
- **Multi-device-as-recovery polish** — encouraging ≥2 linked devices as a secondary safety net (recovery code #4 is the primary answer).
- **Team invite flow beyond tokens** — richer team management (roles, per-vault invites, expiry policy UI) past the basic invite token in #3.
- **Web UI / dashboard** — CLI + TUI only for now.
- **Audit log export / SIEM integration** — audit is read-only in-app today; exporting is a later concern.
- **Server observability** — metrics, structured logs, tracing for operators self-hosting at scale.

---

## Open decisions

- [ ] **Execution order:** by-impact (`1 ✅ → 2 → 3 → 4 → 5`) vs quick-win-first (`2 → 3 → 5 → 4`). Item #1 done either way.
