# devsync TUI — UX Fixes Plan (batch 2)

Hasil grilling 2026-07-21. TUI dasar (PLAN.md) sudah jalan; ini perbaikan UX dari
pemakaian nyata + beberapa quick win. **Semua client-only** — tak sentuh server,
CLI behavior, atau schema. Kerjaan seluruhnya di `internal/client/tui/`.

## Keputusan (hasil grilling)

1. **Esc + filter**: bug — `rootModel` (`app.go:50`) sikat `esc` global sebelum
   sampai list, jadi esc pas filter aktif malah pop/quit TUI. Fix: kalau view top
   stack punya list yang lagi filtering (`list.FilterState() != list.Unfiltered`),
   teruskan esc ke list (clear filter), jangan pop. Esc di top menu tanpa filter
   tetap = quit (opsi a, behavior sekarang dipertahankan).
2. **Add vault**: key `c` di Vaults view → push view pilih team (pakai
   `actions.ListTeams`, pattern list→enter yang sudah ada) → enter → textinput
   nama vault → `actions.CreateVault(name, team)` → pop + status sukses +
   reload vaults. Konsisten sama `c` create di Teams view.
3. **Filepicker (push)**: start tetap cwd. Tambah header yang nampilin
   `CurrentDirectory` (jawab "gak jelas posisi di mana"). Key `~` = lompat ke
   home directory. Navigasi masuk/keluar folder sudah ada bawaan bubbles
   (`enter`/`h`/`backspace`) — cuma tak kelihatan karena tak ada path header.
   Antar-drive (C: ↔ D:) **skip** — tambah nanti kalau kepake.
4. **Devices revoke**: `enter` = revoke itu jebakan + inkonsisten (di view lain
   enter = drill-down). Pindah ke `d` (konsisten sama delete di Files/Teams),
   `enter` jadi no-op. Confirm dialog tetap.
5. **Refresh**: key `r` = reload data di semua list view (vaults, files, teams,
   members, devices, audit log). Sekarang data cuma load sekali pas masuk view.
   Hati-hati: `r` cuma aktif kalau list TIDAK lagi filtering (jangan telan
   ketikan filter).
6. **Status auto-clear**: status message (sukses/error) sekarang nempel
   selamanya. Auto-clear setelah ~4 detik via `tea.Tick` + generation counter
   (increment tiap set status; tick cuma clear kalau generation match, biar
   status baru tak kehapus timer lama).
7. **Spinner**: ganti teks statis `loading…` dengan bubbles `spinner` di semua
   loading state. Satu komponen shared, jangan copy-paste per view.

## Non-goals (eksplisit, jangan dikerjain)

- **Private/public team** — dibahas, dibatalkan user. Jangan bangun.
- **Approve fingerprint prefill** — ketik manual itu BY DESIGN (verifikasi
  out-of-band). Prefill dari server = verifikasi jadi teater. Jangan sentuh.
- Server / CLI / schema changes. Batch ini murni `internal/client/tui/`
  (+ mungkin helper kecil, tapi `internal/client/actions` sudah punya semua
  yang dibutuhin: `CreateVault`, `ListTeams` sudah ada).
- Breadcrumb, mouse support, theme config — tak diminta.

## Catatan implementasi

- `filepicker.New()` (bubbles v1.0.0) default `CurrentDirectory: "."` — item #3
  tinggal set field + render header + handle `~`.
- Pattern view baru contek `teams.go` (`teamInputModel` buat input nama vault,
  `teamsModel` buat list pilih team).
- Item #1: rootModel tak bisa introspeksi list child langsung — opsi paling
  murah: tiap list view expose method/interface `isFiltering() bool`, rootModel
  cek sebelum handle esc. Alternatif: biarkan esc diteruskan ke top view dulu,
  view return "consumed" sentinel. Pilih yang paling kecil diff-nya.
- Verifikasi: `go build ./...` + `go vet ./...` clean, lalu smoke test manual
  dari terminal beneran (butuh real tty): filter→esc, create vault, push file
  lewat picker, `r` refresh, revoke via `d`, status hilang sendiri, spinner
  muter.

## Progress

- [ ] 1 — Fix esc pas filter aktif
- [ ] 2 — Add vault (`c` → pilih team → nama)
- [ ] 3 — Filepicker: header path + `~` home
- [ ] 4 — Devices: revoke pindah ke `d`
- [ ] 5 — `r` refresh semua list
- [ ] 6 — Status auto-clear 4 detik
- [ ] 7 — Spinner loading
