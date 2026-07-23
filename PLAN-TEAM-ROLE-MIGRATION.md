# Plan: Team-Scoped Roles (Drop Global Admin)

## Decisions (locked)

1. **Role hanya di `team_members`.** Kolom baru `role IN ('admin','member')`. Tidak ada per-vault role (YAGNI). Team admin = atur semua vault di tim itu.
2. **Bootstrap tetap, tanpa admin flag.** First user POST `/bootstrap` → status=active only. Tidak promote admin apa pun. Kondisi `CountActiveUsers == 0`.
3. **Hapus global approve.** User baru gabung hanya via invite token. Tidak ada `/admin/approve`, tidak ada `/teams/join` (pending join by name dibuang).
4. **WhoAmI**: drop `IsAdmin`, add `TeamRoles []{Team, Role}`.
5. **Path rename**: semua `/admin/*` (kecuali bootstrap) → `/teams/*` namespace. Bersih, tidak misleading.
6. **Migration**: DB bersih, edit langsung `0001_init.sql` + buat migration `0002_drop_global_admin.sql`.

## Scope

Hapus global admin. Role admin/member hidup hanya di `team_members`. User global status tetap `pending/active/disabled` untuk gating device + bootstrap.

---

## A. Schema

### `internal/db/migrations/postgres/0001_init.sql`
- Drop `users.is_admin` kolom + drop comment "admin approves" jadi "active required".
- `team_members`: tambah `role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('admin','member'))`. Update comment (hapus "reserved for future finer roles").
- Ganti comment line 4: "status: pending until active (via invite token atau bootstrap first-user)".

### `internal/db/migrations/sqlite/0001_init.sql`
- Sinkron sama postgres: drop `is_admin INTEGER`, add `role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('admin','member'))` di team_members.

### Migration baru: `0002_drop_global_admin.sql` (postgres + sqlite)
- Untuk existing deploy: `ALTER TABLE users DROP COLUMN is_admin;` `ALTER TABLE team_members ADD COLUMN role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('admin','member'));`
- Asumsi ponytail: user bilang "hapus semua data" — jadi aman. Tetap sediakan migration untuk kelengkapan.

---

## B. Store (`internal/server/store/`)

### `models.go`
- `User` struct: hapus field `IsAdmin bool`.
- `Member` struct: tambah `Role string`.

### `users.go`
- Hapus fungsi `CountAdmins`, `SetUserAdmin`.
- `GetUserByUsername`: query drop `is_admin` + drop scan field.
- `GetUserByFingerprint`: sama.
- `ListAllUsers` (jika ada): drop `is_admin`.
- Tambah: `CountActiveUsers(ctx) (int, error)` — `SELECT count(*) FROM users WHERE status='active'`. Untuk bootstrap gate.

### `teams.go`
- `AddTeamMember(ctx, teamID, userID, status string, role string)` — signature +1 param. Query INSERT + role column.
- `ListMembers`: SELECT tambah `m.role`, scan ke `Member.Role`.
- `ListTeamsForUser`: SELECT tambah `m.role` + return `[]TeamWithRole` atau attach ke `Member`. Alternatif: struct baru `UserTeam{Team, Role}`. Rec: tambah method `ListTeamsWithRoleForUser(ctx, userID) ([]TeamWithRole, error)` returning `(Team, Role)` per row. Pakai di WhoAmI.
- Tambah `IsTeamAdmin(ctx, teamID, userID) (bool, error)` — `SELECT 1 FROM team_members WHERE team_id=? AND user_id=? AND role='admin' AND status='active'`.
- Tambah `SetTeamMemberRole(ctx, teamID, userID, role)`.
- Hapus `ActivatePendingMemberships` (pending join dibuang).

### `invite.go` (cek jika ada `ClaimInviteToken`)
- `ClaimInviteToken`: set `users.status='active'` + insert/update `team_members (team_id, user_id, status='active', role='member')`.
- Pastikan claim token tidak pernah kasih role='admin'.

---

## C. HTTP Routes (`internal/server/http/server.go`)

Rename mux registrations:

```
POST /bootstrap                              rateLimit(s.handleBootstrap)            // no auth
POST /teams/create                           s.activeAuthed(s.handleCreateTeam)
POST /teams/delete                           s.teamAdminAuthed(s.handleDeleteTeam)
POST /teams/vaults/create                    s.teamAdminAuthed(s.handleCreateVault)
POST /teams/vaults/grant                     s.teamAdminAuthed(s.handleGrant)
POST /teams/vaults/revoke                    s.teamAdminAuthed(s.handleRevoke)
POST /teams/invite                           s.teamAdminAuthed(s.handleInvite)
POST /teams/set-admin                        s.teamAdminAuthed(s.handleSetAdmin)
GET  /teams                                  s.activeAuthed(s.handleTeams)
GET  /teams/members                          s.activeAuthed(s.handleMembers)
POST /teams/claim                            s.authed(s.handleClaimInvite)          // pending OK
```

Hapus:
- `POST /admin/approve`
- `POST /admin/set-admin` (ganti dengan `/teams/set-admin`, team-scoped)
- `POST /teams/join`

---

## D. Auth Middleware (`internal/server/http/auth.go`)

- Hapus `adminAuthed`.
- Tambah:
  ```go
  func (s *Server) teamAdminAuthed(next http.HandlerFunc) http.HandlerFunc {
      return s.activeAuthed(func(w http.ResponseWriter, r *http.Request) {
          var body struct{ Team string `json:"team"` }
          if json.Unmarshal(bodyOf(r), &body) != nil || body.Team == "" {
              writeErr(w, http.StatusBadRequest, "team required")
              return
          }
          t, err := s.store.GetTeamByName(r.Context(), body.Team)
          if err != nil { writeErr(w, http.StatusNotFound, "team not found"); return }
          ok, err := s.store.IsTeamAdmin(r.Context(), t.ID, userOf(r).ID)
          if err != nil || !ok { writeErr(w, http.StatusForbidden, "team_admin only"); return }
          next(w, r)
      })
  }
  ```
  Catatan: untuk endpoint yang body-nya tidak punya field `team` (mis. `CreateTeam`), pakai `activeAuthed` langsung. Untuk `CreateVault`, `Grant`, `Revoke`, `Invite`, `SetAdmin`, `DeleteTeam` — field `team` wajib di body.

---

## E. Handlers

### `handlers_identity.go`
- `handleRegister`: hapus branch `if existing.IsAdmin` (line 67-72). Hanya link-signature path yang activate. Device pertama user selalu `pending` kecuali via invite-claim (lihat `handleClaimInvite`).
- `handleWhoAmI`: drop `IsAdmin` field. Tambah `TeamRoles`:
  ```go
  teams, err := s.store.ListTeamsWithRoleForUser(r.Context(), u.ID)
  // build []protocol.TeamRole{Team, Role}
  writeJSON(w, 200, protocol.WhoAmIResponse{
      Username: u.Username, Status: u.Status,
      Device: ...,
      TeamRoles: roles,
  })
  ```

### `handlers_teams.go`
- `handleCreateTeam`: creator `AddTeamMember(ctx, t.ID, creator.ID, "active", "admin")`.
- `handleDeleteTeam`: body tetap `{name}`. Middleware `teamAdminAuthed` butuh field `team` — handler map `team = req.Name`. Solusi: ubah DTO `CreateTeamRequest` jadi `{Name, Team string}` with `Team` optional, atau tambah helper `teamAdminAuthedByName(next, nameField)`. Rec: ganti body pakai `{team: "..."}` konsisten di seluruh admin endpoint. Update `protocol.CreateTeamRequest` field `Name` → `Team`.
- `handleSetAdmin`: rewrite:
  ```go
  var req struct{ Team, Username string }
  // validate caller team_admin (middleware sudah)
  // cek target member active di tim itu
  // SetTeamMemberRole(ctx, t.ID, target.ID, "admin")
  ```
- `handleInvite`: body `{team, username}` (ganti dari `TeamName` → `Team`).
- `handleJoin`: hapus fungsi + route.
- `handleApprove`: hapus fungsi + route.
- `handleMembers`: include `Role` di response `protocol.Member`.

### `handlers_vaults.go`
- Auth via middleware. Body request harus ada field `team`. Update DTO `CreateVaultRequest`, `GrantRequest`, `RevokeRequest` field `TeamName`/`Vault` → `Team` + `Vault`.

### `server.go` `handleBootstrap`
- Hapus `SetUserAdmin(ctx, user.ID, true)`.
- Ganti gate: `CountAdmins == 0` → `CountActiveUsers == 0`.
- Tetap activate user + device.
- Dipindah route ke `POST /bootstrap`.

---

## F. Protocol DTO (`internal/protocol/dto.go`)

- `WhoAmIResponse`: hapus `IsAdmin bool`. Tambah `TeamRoles []TeamRole`.
- `TeamRole` struct baru: `{Team string; Role string}`.
- `ApproveRequest`: hapus.
- `Member`: add `Role string`.
- `CreateTeamRequest`: field `Name` → `Team` (rename untuk konsistensi middleware).
- `CreateVaultRequest`, `GrantRequest`, `RevokeRequest`, `InviteRequest`: pastikan ada field `Team` (rename dari `TeamName`).

---

## G. Client

### Hapus
- `internal/client/actions/approve.go` — hapus file.
- `internal/client/commands/approve.go` — hapus file.
- `internal/client/commands/root.go`: buang `newApproveCmd()` dari root.

### Edit
- `internal/client/commands/register.go`: hapus branch `if resp.IsAdmin` (line ~62). Print universal: "Ask team admin for an invite token, then run `devsync teams join <token>`."
- `internal/client/commands/setup.go`: hapus isAdmin loop + check bootstrap-admin repeat. Cek status via `/whoami`; lanjut setup kalau active.
- `internal/client/commands/bootstrap.go`: ganti URL `/admin/bootstrap` → `/bootstrap`. Long help tetap.
- `internal/client/commands/helptopics.go`: update onboarding steps:
  1. devsync init
  2. devsync register
  3. devsync bootstrap-admin (first user only, no admin promotion)
  4. devsync teams create <name>
  5. devsync teams invite <user> --team <name> → share token
  6. teammate: devsync teams join <token>
- `internal/client/commands/device.go`: update mention fallback "use invite token / team admin" (bukan `devsync approve`).
- `internal/client/commands/teams.go` (`join` cmd): dukung hanya token. Drop "request join by name" path. Token → claim via `/teams/claim`. Output: "joined team X".
- `internal/client/teams` URL constants: update semua `/admin/*` → `/teams/*` namespace baru.
- `internal/client/actions/vaults.go` `CreateVault`, `Grant`, `Revoke`: pass field `Team` (rename dari `TeamName`).
- `internal/client/actions/teams.go` `Invite`: pass `Team`.
- `internal/client/actions/onboard.go` `BootstrapAdmin`: rename ke `BootstrapActiveUser` (opsional, biar semantik jelas). URL update ke `/bootstrap`. Tetap first-user gate.

### TUI
- `internal/client/tui/wizard.go`: ganti `isAdmin` flag → `isActive` (cek `user.status == "active"`). Auto-flow: register → bootstrap (first only) → cek active → unlock.
- `internal/client/tui/waiting.go`: message "Waiting for an invite token from your team admin." + hint "Ask for `devsync teams invite`, then `devsync teams join <token>`." Bukan "admin approve".
- `internal/client/tui/teams.go`:
  - Drop "approve selected" menu item.
  - Pending approval text `[pending approval]` → `[pending invite]`.
  - Join request text `joined team X (pending approval)` → hapus (claim token langsung active).
  - (Opsional) Tambah menu "promote to team_admin" untuk team_admin view.
- `internal/client/tui/menu.go`: pending count text `N pending approval` → `N pending invites` (atau hapus counter total).

---

## H. Tests

### `internal/server/http/integration_test.go`
Rewrite setup:
```go
// first user registers + bootstraps active (no admin)
admin := newTestDevice(t)
registerDevice(t, ts.URL, "alice", admin)
// call /bootstrap directly (skip, simulating CLI)
// atau: st.SetUserStatus(ctx, aliceUser.ID, "active") — manual, no admin flag

// create team → alice auto team_admin
admin.post(t, ts.URL, "alice", "/teams/create", protocol.CreateTeamRequest{Team: "eng"}, &team)

// create vault as team_admin
admin.post(t, ts.URL, "alice", "/teams/vaults/create", ...)

// invite second user
admin.post(t, ts.URL, "alice", "/teams/invite", protocol.InviteRequest{Team: "eng", Username: "budi"}, &inviteResp)

// budi registers + claims invite → active + member
budi := newTestDevice(t)
registerDevice(t, ts.URL, "budi", budi)
budi.post(t, ts.URL, "budi", "/teams/claim", protocol.ClaimInviteRequest{Token: inviteResp.Token}, nil)

// budi whoami → status active, TeamRoles has {eng, member}
// alice grants budi vault access
admin.post(t, ts.URL, "alice", "/teams/vaults/grant", protocol.GrantRequest{Team: "eng", Vault: "secrets", Username: "budi"}, nil)

// budi pull → succeeds
```

Hapus semua test yang:
- panggil `SetUserAdmin`
- pakai `/admin/approve`
- pakai `/teams/join` by name
- pakai `IsAdmin` assertion di whoami

Tambah test baru:
- team_admin check: non-admin member coba `/teams/vaults/grant` → 403.
- admin di tim A coba grant vault tim B → 403.
- bootstrap gagal kalau sudah ada active user.
- bootstrap sukses kalau 0 active user.

---

## I. Docs

- `README.md`: update onboarding section + command table (`approve` row hapus, `join` jelas hanya token).
- `SECURITY.md`: update threat model row "Can approve malicious devices as admin" → "Can issue invite tokens (team-admin only), can tamper team_members.role".
- `PLAN.md`, `PLAN-ONBOARDING-UX.md`, `PLAN-TUI-UX.md`, `ROADMAP.md`: update bagian yang mention global admin approve.

---

## Verification

1. `go build ./...`
2. `go vet ./...`
3. `go test ./...`
4. Manual smoke (docker-compose up):
   - First user: register → bootstrap (`/bootstrap`) → status active.
   - Create team `/teams/create` → whoami TeamRoles has `{team, admin}`.
   - Invite token → second user register → claim token → active + member.
   - Team admin grant vault → second user pull file OK.
   - Non-admin member coba delete team → 403.
   - Bootstrap second time (active user > 0) → 403.
5. Lint: `gofmt -l .` (harus kosong), `goimports -l .`.

---

## Ponytail notes
- Skip per-vault role, warisi team. Vault admin = team admin.
- Pending-join dibuang, hanya invite token. 2-3 fungsi + 1 endpoint hilang.
- Path `/teams/*` namespace; investasi client URL update sekali, jelas selamanya.
- DB bersih, no data migration pain.
- `ponytail: CountActiveUsers gate` — cukup untuk bootstrap, tidak perlu CountUsers (pending user butuh invite-claim yang butuh active call khusus).
