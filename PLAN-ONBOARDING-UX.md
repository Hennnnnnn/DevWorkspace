# PLAN: Onboarding & UX Simplification

Hasil grilling session 2026-07-22. Semua keputusan sudah dikonfirmasi user.
Tujuan: `devsync` = satu entry point, onboarding <30 detik, pemula tidak pernah
lihat `init`/`register`/`bootstrap-admin`/konsep team-vault.

---

## Keputusan final

### Onboarding (wizard di TUI)

1. **`devsync` tanpa argumen = satu-satunya entry point.**
   State machine saat launch:
   - Belum ada keystore / belum register → wizard onboarding di TUI
   - Status pending → layar "Waiting for Approval"
   - Normal → TUI seperti sekarang

2. **Wizard hanya tanya 2 input: username + passphrase.**
   Sisanya otomatis, urutan:
   1. init (generate keypair) — passphrase dari input wizard
   2. register — username dari input wizard
   3. bootstrap-admin (dicoba diam-diam)
   4. unlock — pakai passphrase yang sama, JANGAN tanya dua kali
   5. auto-create team `personal` + vault `main` (hanya jika bootstrap-admin sukses)
   - Username bentrok (sudah dipakai): pesan jelas, tanya ulang username SAJA,
     jangan ulang seluruh wizard.
   - Target: selesai <30 detik, langsung bisa `devsync push` / `devsync pull`.
   - Jangan expose istilah team/vault di wizard.

3. **Bootstrap-admin gagal = alur teammate, BUKAN error.**
   - Skip auto-create team/vault.
   - Tampilkan layar "Waiting for Approval": username + fingerprint mudah
     di-copy + instruksi kirim ke admin.

4. **Layar waiting auto-poll `/whoami`** (interval ~5 detik).
   Begitu approved → transisi otomatis ke TUI normal tanpa restart.
   TIDAK ada command baru (`login`/`sync` tidak dibuat).

5. **Layar recovery phrase khusus di wizard** (KRITIS — jangan disederhanakan):
   - Tampilkan 24 kata mnemonic.
   - Peringatan tebal: "hanya ditampilkan SATU KALI, satu-satunya cara pulihkan
     akses jika passphrase/device hilang".
   - Konfirmasi eksplisit ("Saya sudah menyimpannya") sebelum lanjut.
   - TANPA verifikasi ketik-ulang kata (backlog).

6. **Server URL tidak ditanya.** Pakai baked-in `DefaultServerURL`.
   Tampilkan sebagai info kecil di layar awal wizard
   ("Server: https://... — ganti: devsync config set server_url").

### Sisi admin

7. **Approve + grant = satu alur di TUI:**
   - Badge pending count di menu.
   - Pilih user pending → layar konfirmasi tampilkan username + fingerprint
     (verifikasi visual, keamanan — jangan dihilangkan).
   - Approve → langsung picker "grant ke vault mana?" (multi-select,
     default: semua vault team, bisa diubah).
   - Eksekusi approve + grant bersamaan. Teammate langsung bisa pull.

### CLI harian

8. **`--vault` jadi opsional di `push`/`pull`/`history`/dll:**
   - User punya 1 vault → pakai otomatis, tanpa tanya.
   - >1 vault → picker interaktif (TTY) / error jelas + daftar vault (non-TTY).

9. **`devsync pull` tanpa argumen file:**
   - 1 file di vault → langsung pull.
   - >1 file → picker interaktif (TTY) / error jelas + daftar file (non-TTY).
   - JANGAN pernah overwrite file lokal diam-diam.

10. **Istilah `push`/`pull` dipertahankan.**
    Perjelas deskripsi di TUI + help text:
    - "push = encrypt & upload"
    - "pull = download & decrypt"

11. **Default unlock TTL: 15 menit → 8 jam.**
    Lokasi: `internal/client/commands/unlock.go:29` (flag `--timeout`).
    Pesan prompt unlock: jelaskan "membuka kunci device", bukan "login ulang".

12. **Help dikelompokkan (Cobra groups):**
    - "Daily" di atas: push, pull, history, checkout, rm, audit.
    - "Advanced/Setup" di bawah: init, register, unlock, config, dst. dengan
      catatan "biasanya tidak perlu — jalankan `devsync` saja".
    - `bootstrap-admin` + `setup` → `Hidden: true` (tetap berfungsi).
    - Semua command lama tetap ada (kompatibilitas script/CI).

---

## Backlog (sadar ditunda, JANGAN kerjakan sekarang)

- Rename team/vault (tidak ada endpoint; nama `personal`/`main` cukup).
- Per-folder vault memory (ingat pilihan vault per direktori).
- OS keychain / "remember me" di unlock.
- Verifikasi ketik-ulang 3 kata recovery phrase.

---

## Fakta codebase relevan (verified 2026-07-22)

- `internal/client/commands/root.go` — semua command terdaftar; `NewRoot()`.
- `internal/client/commands/setup.go` — wizard CLI lama (akan di-hidden,
  logikanya jadi referensi wizard TUI).
- `internal/client/commands/init.go` — generate keypair + 24-word mnemonic
  (`crypto.GenerateRecoverySeed` → `SeedToMnemonic`), fingerprint.
  Mnemonic hanya bisa ditampilkan saat pembuatan.
- `internal/client/commands/unlock.go:29` — default TTL 15 menit.
- `internal/client/tui/app.go` — rootModel stack; `Init()` sudah auto-push
  unlock view kalau locked. First-run detection masuk di sini.
- `internal/client/actions/` — business logic dipakai CLI + TUI berdua.
  Wizard TUI harus panggil actions, bukan duplikasi logic.
- `keystore.Exists()` + `config.Load()` (`cfg.Username == ""`) = deteksi first-run.
- TUI: Bubble Tea, drill-down stack (lazygit-style), esc = back.
- Entry: `devsync` tanpa arg → TUI (`tui.Run()`), dengan arg → Cobra CLI.

## Urutan implementasi yang disarankan

1. TTL default 8 jam (satu baris, `unlock.go:29`).
2. Vault resolution helper di `actions` (1 vault → auto; picker; non-TTY error)
   + pull tanpa argumen file.
3. Wizard TUI first-run (state detection di `app.go` / `newRootModel`):
   layar username+passphrase → recovery phrase confirm → auto steps →
   waiting-approval (poll) ATAU langsung menu.
4. Approve+grant satu alur di TUI members view.
5. Help grouping + hidden commands + perjelas deskripsi push/pull.
