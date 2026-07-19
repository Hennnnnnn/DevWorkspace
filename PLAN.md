# devsync TUI — Implementation Plan

Base CLI (vaults/teams/devices/files/audit, E2E crypto, agent unlock) sudah selesai. Plan ini nambahin TUI di atas CLI yang ada, tanpa ubah behavior CLI.

## Keputusan (hasil grilling)

1. **Scope**: Full TUI — semua fitur (vaults, teams, devices, files, audit, approvals). CLI tetap ada, tak berubah.
2. **Framework**: `charmbracelet/bubbletea` (core) + `charmbracelet/bubbles` (widgets: list, textinput, filepicker, spinner, viewport) + `charmbracelet/lipgloss` (styling).
3. **Invocation**: bare `devsync` (tty terdeteksi via `term.IsTerminal`) → launch TUI. `devsync <subcommand>` tetap CLI biasa, scriptable, tak berubah. Non-tty (piped) tanpa subcommand → fallback ke help seperti sekarang.
4. **Locked state**: degrade per-view. View yang butuh vault key decrypt (file content, push, pull) tampil state "locked — press U to unlock" kalau agent locked. View lain (teams, vaults list, devices, audit log — metadata only) langsung jalan tanpa perlu unlock.
5. **Code sharing**: extract business logic dari tiap `RunE` closure ke `internal/client/actions` package (plain functions, no cobra dependency). Cobra `RunE` jadi thin wrapper: panggil action, print hasil. TUI panggil fungsi yang sama.
6. **Progress reporting**: tiap action multi-step (push, pull, dll) terima `onStep func(msg string)` callback param. CLI pass callback yang drive `startSpinner` (existing). TUI pass callback yang `program.Send(stepMsg{msg})` ke model.
7. **Navigasi**: drill-down stack (gaya lazygit/k9s). Top menu → list → detail → actions. Esc = pop stack (back). Data hierarchical (team → vault → file → versions), drill-down match struktur ini.
8. **File picker (push)**: pakai Bubbles `filepicker` widget — full filesystem navigation, bukan text input manual.
9. **Destructive actions confirm**: TUI only. `rm`, `delete-team`, `device revoke`, vault `revoke` tampil dialog "Delete X? (y/N)" sebelum eksekusi. CLI tetap immediate/no-confirm (scripting contract tak berubah).
10. **Styling**: full semantic theme via Lipgloss — success (hijau) / danger (merah, buat confirm destructive) / warning (kuning, buat locked state) / info (biru) / selection (accent color buat highlight list item aktif).
11. **Package layout**: `internal/client/tui/` — satu file per view (`vaults.go`, `teams.go`, `devices.go`, `files.go`, `audit.go`, `filepicker.go`, `confirm.go`, `unlock.go`, `theme.go`, `app.go` sbg root model). Import `internal/client/actions`.
12. **Dependencies baru** (tambah ke go.mod): `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/bubbles`, `github.com/charmbracelet/lipgloss`.

## Progress

- [x] Fase 1 — Extract actions package (`internal/client/actions/`, semua RunE jadi thin wrapper). `go build`/`go vet` clean.
- [x] Fase 2 — Deps + skeleton (`go get` bubbletea/bubbles/lipgloss, `tui/theme.go`, `tui/app.go` root model, wired `cmd/devsync/main.go`: no-args+tty → TUI, else CLI).
- [x] Fase 3 — Top-level menu + drill-down stack (`tui/stack.go` push/pop msg, `tui/menu.go` top menu list Vaults/Teams/Devices/Audit, `tui/placeholder.go` stub views, `tui/app.go` rootModel stack, esc=back/quit, ctrl+c=quit, WindowSizeMsg propagate ke stack).
- [x] Fase 4 — Vaults + Files view (`tui/vaults.go` list vault→files; `tui/files.go` files list + history detail; `actions.ListVaults` baru; enter=drill, p=pull, u=push (filepicker), d=rm, c=checkout; async load via tea.Cmd, status banner). `go build`/`go vet` clean.
- [x] Fase 5 — Locked-state handling (`tui/unlock.go` masked textinput view; `actions.IsUnlocked` baru; files+history view: banner "locked — press U", key U buka unlock, pull/checkout guard IsUnlocked; unlock sukses = popView balik). `go build` clean.
- [x] Fase 6 — Teams view
- [x] Fase 7 — Devices view
- [x] Fase 8 — Audit view
- [x] Fase 9 — Confirm dialog component
- [x] Fase 10 — Polish
- [ ] Fase 11 — Testing

Belum smoke-test interaktif TUI beneran (perlu real tty) — cuma verify via `go build`/`go vet`. Test manual dari terminal user sendiri masih pending.

## Fase Build (urutan usul)

1. **Extract actions package** (`internal/client/actions/`): pindah logic dari `RunE` di `files.go`, `teams.go`, `vaults.go`, `device.go`, `register.go`, `approve.go`, `unlock.go` ke fungsi plain dengan `onStep` callback. Refactor semua `RunE` existing jadi thin wrapper panggil fungsi ini + print via `startSpinner`. Verifikasi: `go build`, semua CLI command jalan sama seperti sebelum (manual smoke test tiap command).
2. **Deps + skeleton**: `go get` tiga package Bubble Tea. Bikin `internal/client/tui/app.go` — root model kosong (cuma render "hello"), `theme.go` dengan 5 warna semantic via Lipgloss. Wire `cmd/devsync/main.go`: kalau no-args + `term.IsTerminal(stdout)` → `tui.Run()`, else `commands.NewRoot().Execute()`.
3. **Top-level menu + drill-down mekanisme**: model stack pattern (push/pop view model on selection/Esc). Top menu: Vaults / Teams / Devices / Audit.
4. **Vaults + Files view** (paling kompleks, prioritas dulu): list vault → list file per vault (pakai `actions.ListFiles`) → detail file (history) → actions (pull, checkout, rm+confirm). Push pakai `filepicker` widget → confirm vault target → jalanin `actions.Push` dgn step callback.
5. **Locked-state handling**: cek agent status di file view; kalau locked, tampil banner + key `U` buka `unlock.go` view (passphrase input pakai `bubbles/textinput` masked).
6. **Teams view**: list teams → members (+ pending) → approve (fingerprint input) → create-team/delete-team (delete pakai confirm dialog).
7. **Devices view**: list devices → revoke (confirm dialog) → add device flow.
8. **Audit view**: read-only list, no drill-down lebih dalam dari vault → entries.
9. **Confirm dialog component**: generic reusable `confirm.go` model dipanggil dari rm/delete-team/revoke/device-revoke.
10. **Polish**: keybinding help footer tiap view, error display (toast/banner style), window resize handling.
11. **Testing**: manual QA tiap view + tiap action end-to-end (push→pull roundtrip, approve flow, revoke flow) di real server/vault test data.

## Non-goals (eksplisit skip, jangan overbuild)

- Tak ada TUI buat `config`, `init`, `register`, `bootstrap-admin`, `set-admin`, `update`, `guide` — command setup/onetime ini tetap CLI-only, jarang dipakai interaktif berulang.
- Tak ada theme customization/config (warna hardcoded di `theme.go`, bukan user-configurable).
- Tak ada mouse support — keyboard-only nav (drill-down + confirm dialogs).
