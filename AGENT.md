# AGENT.md — Panduan untuk AI Coding Agent

> Baca file ini **sepenuhnya** sebelum menulis kode apa pun. Versi dokumen: 1.1 (2026-11-14).

## 1. Tentang Proyek

**LUMA Monitoring** adalah dashboard monitoring server custom untuk ±60 server Linux.

- **Agent** (bash, sudah ada di `agent/`) mengirim metrik tiap menit ke backend.
- **Backend** (Go, satu binary) menerima data, menyimpan ke **SQLite**, menjalankan alert, mengirim notifikasi **Telegram dan Email**, dan melayani frontend.
- **Frontend** (React + TypeScript) adalah dashboard, di-*embed* ke binary Go.
- **Deploy**: satu container Docker di **Coolify** dengan persistent volume `/data`.

Bahasa dokumen: Indonesia. Bahasa kode, komentar kode, nama field API, dan pesan commit: **Inggris**. Teks UI dan pesan notifikasi: **Indonesia**.

## 2. Aturan Emas (tidak boleh dilanggar)

1. **Jangan mengarang.** Nama field, tabel, kolom, endpoint, env var, dan library hanya boleh yang tertulis di `docs/`. Jika tidak ada, lihat bagian 12.
2. **Agent script adalah kebenaran mutlak untuk format payload.** Jika `docs/PROTOCOL.md` tampak bertentangan dengan `agent/luma_agent.sh`, ikuti script dan laporkan selisihnya.
3. **Kerjakan satu tugas dari `docs/TASKS.md` pada satu waktu**, sesuai urutan. Jangan melompat atau menggabungkan tugas.
4. **Semua SQL memakai parameter** (`?`). Tidak ada penggabungan string untuk nilai dari luar.
5. **Isi payload agent adalah data tak tepercaya.** Validasi, batasi ukuran, dan jangan pernah dieksekusi atau ditampilkan tanpa escape.
6. **Secret tidak pernah dicatat ke log, dikirim ke frontend, atau di-commit** (SID, token bot, password SMTP, `LUMA_SECRET_KEY`, password).
7. **Hanya satu goroutine penulis ke SQLite.** Jangan membuka koneksi tulis lain.
8. **Semua timestamp adalah integer detik Unix (UTC).** Konversi zona waktu hanya di frontend.
9. **Tidak ada dependensi baru di luar allowlist** (`docs/CONVENTIONS.md` bagian 7) tanpa entri baru di `docs/DECISIONS.md`.
10. **Setiap perubahan kontrak** (payload, skema DB, API) wajib memperbarui dokumen terkait **dalam commit yang sama**.

## 3. Sumber Kebenaran dan Peta Dokumen

Jika dua dokumen berbeda, yang lebih atas dalam tabel ini menang.

| Prioritas | Dokumen | Isi |
|---|---|---|
| 1 | `agent/*.sh`, `agent/luma.cfg` | Perilaku agent yang sebenarnya |
| 2 | `docs/PROTOCOL.md` | Format payload, endpoint agent, aturan parsing |
| 3 | `docs/DATABASE.md` | Skema SQLite, indeks, aturan migrasi |
| 4 | `docs/API.md` | Kontrak REST untuk frontend |
| 5 | `docs/DESIGN.md` | Arsitektur, alert engine, notifikasi, keamanan |
| 6 | `docs/FRONTEND.md` | Halaman, komponen, grafik, UX |
| 7 | `docs/DEPLOYMENT.md` | Docker, Coolify, env var, distribusi agent, backup |
| 8 | `docs/CONVENTIONS.md` | Gaya kode, testing, allowlist dependensi |
| 9 | `docs/DECISIONS.md` | Keputusan terkunci dan pertanyaan terbuka |
| 10 | `docs/TASKS.md` | Daftar tugas berurutan dengan kriteria selesai |
| - | `docs/PROMPTS.md` | Template prompt untuk pengguna manusia |

## 4. Stack (terkunci)

| Lapisan | Pilihan |
|---|---|
| Bahasa backend | Go ≥ 1.22 (target 1.23), modul `github.com/nocturnalismee/luma-monitoring/backend` |
| HTTP | `net/http` stdlib (`ServeMux` dengan pola metode+path Go 1.22), **tanpa framework** |
| Database | SQLite (WAL) lewat `modernc.org/sqlite` (tanpa CGO) |
| Logging | `log/slog` (JSON) |
| Frontend | React 18, Vite, TypeScript strict, Tailwind CSS, shadcn/ui, TanStack Query, React Router 6, Zustand, react-hook-form + zod, Recharts, lucide-react, cmdk |
| Container | Dockerfile multi-stage, image akhir `alpine` |
| Deploy | Coolify (proxy HTTPS disediakan Coolify) |
| Backup | Litestream ke S3-compatible |

## 5. Struktur Repo

```
luma-monitoring/
├── AGENT.md
├── Makefile
├── agent/                    # script agent (dimodifikasi minimal, lihat DEPLOYMENT.md)
│   ├── luma_agent.sh  luma.cfg  luma_install.sh  luma_update.sh  luma_uninstall.sh
├── backend/
│   ├── cmd/luma/main.go
│   ├── internal/
│   │   ├── config/  db/  store/  ingest/  parser/  writer/
│   │   ├── api/  auth/  alerts/  notify/  jobs/  crypto/  agentdist/  web/
│   ├── migrations/           # 0001_init.sql, ... (di-embed)
│   └── testdata/payloads/    # fixture payload nyata
├── frontend/
│   └── src/ (app, components, features, lib, hooks)
├── docs/                     # dokumen ini
├── scripts/                  # replay payload, load test
└── Dockerfile
```

## 6. Perintah

Target `make` dibuat pada tugas **T-000**. Sebelum itu, jangan mengasumsikan perintah ini ada.

| Perintah | Fungsi |
|---|---|
| `make setup` | Instal dependensi backend dan frontend |
| `make dev-backend` | Jalankan backend (`go run ./cmd/luma`) dengan env dev |
| `make dev-frontend` | Vite dev server (proxy `/api` ke backend) |
| `make test` | `go test -race ./...` + `npm run test` |
| `make lint` | `gofmt` check, `go vet ./...`, `npm run lint`, `npm run typecheck` |
| `make build` | Build frontend lalu binary Go |
| `make docker` | Build image Docker |
| `make replay FILE=...` | Kirim payload fixture ke backend lokal |

## 7. Alur Kerja per Tugas

1. Buka `docs/TASKS.md`, ambil tugas **pertama yang belum selesai** (`[ ]`) yang prasyaratnya sudah `[x]`.
2. Baca semua dokumen di kolom **Refs** tugas itu. Jangan mengandalkan ingatan.
3. Tulis rencana singkat (file yang disentuh, fungsi baru, tes) **sebelum** koding.
4. Implementasikan dengan perubahan sekecil mungkin. Tulis tes bersamaan dengan kode.
5. Jalankan `make lint` dan `make test`. Perbaiki sampai hijau.
6. Perbarui dokumen yang terdampak, lalu centang tugas di `docs/TASKS.md`.
7. Berikan ringkasan: apa yang dibuat, file yang berubah, cara memverifikasi, hal yang belum pasti.

Satu tugas = satu commit (atau satu PR kecil). Format commit: `feat(ingest): decode gzip payload with size limit` (Conventional Commits).

## 8. Konvensi Inti (ringkas)

Detail lengkap ada di `docs/CONVENTIONS.md`.

- Go: `gofmt`, error dibungkus `fmt.Errorf("...: %w", err)`, `context.Context` sebagai argumen pertama, tanpa variabel global, dependensi lewat konstruktor, waktu lewat interface `Clock` agar dapat diuji.
- Handler HTTP tipis: validasi, panggil service/store, tulis respons. Logika bisnis di luar handler.
- Parser adalah **fungsi murni** (input string → struct), tanpa I/O, wajib table-driven test.
- Frontend: TypeScript `strict`, dilarang `any`, data server hanya lewat TanStack Query, tidak ada `fetch` langsung di komponen.
- Format error API seragam (lihat `docs/API.md` bagian 1).

## 9. Keamanan (wajib)

- Ingest: batasi body (`LUMA_INGEST_MAX_BODY`), batasi hasil dekompresi gzip (`LUMA_INGEST_MAX_DECOMPRESSED`), validasi SID **sebelum** parsing berat.
- SID disimpan sebagai **SHA-256 hex**, tidak pernah mentah. Tampilkan SID mentah **sekali** saat dibuat.
- Password: argon2id. Sesi: cookie `HttpOnly`, `Secure`, `SameSite=Strict`, token acak disimpan sebagai hash.
- Secret kanal notifikasi dienkripsi AES-256-GCM. API hanya mengembalikan nilai bertopeng `********`.
- Escape semua teks dinamis di pesan Telegram (HTML) dan email.
- Endpoint tes kirim dan pengaturan: hanya `admin`, dengan rate limit.

## 10. Yang Dilarang

- Menambah framework web (Gin, Echo, Fiber, chi), ORM (GORM, ent), atau library state frontend lain tanpa ADR.
- Mengubah perilaku agent selain perubahan URL yang tercantum di `docs/DEPLOYMENT.md` bagian 4.
- Memakai `localStorage` untuk token autentikasi.
- Menulis migrasi yang mengubah migrasi lama. Selalu tambah file migrasi baru.
- Membuat endpoint atau kolom "sekalian berguna" yang tidak diminta tugas.
- Menghapus atau melemahkan tes agar hijau.
- Memakai JWT. Sesi memakai cookie dan tabel `sessions`.

## 11. Definition of Done

Sebuah tugas selesai hanya jika **semua** terpenuhi:

- [ ] Kriteria penerimaan (*Accept*) di `docs/TASKS.md` terpenuhi.
- [ ] `make lint` dan `make test` hijau (bukan hanya yang baru).
- [ ] Ada tes untuk jalur normal **dan** minimal dua kasus tepi/gagal.
- [ ] Tidak ada secret di log, respons API, atau fixture.
- [ ] Dokumen yang terdampak sudah diperbarui.
- [ ] Tidak ada `TODO` tanpa nomor tugas atau entri di `DECISIONS.md`.
- [ ] Ringkasan hasil ditulis sesuai bagian 7 langkah 7.

## 12. Jika Tidak Yakin

Jangan menebak. Lakukan urutan ini:

1. Cari jawabannya di `docs/` dan di `agent/*.sh` (gunakan `grep`).
2. Jika masih belum jelas, **berhenti**, tulis pertanyaan di bagian "Pertanyaan Terbuka" pada `docs/DECISIONS.md`, lalu tanyakan kepada pengguna.
3. Jika harus melanjutkan, pilih opsi **paling sederhana dan paling aman**, catat asumsinya di ringkasan, dan tandai dengan `// ASSUMPTION(T-xxx): ...`.

## 13. Glosarium Singkat

| Istilah | Arti |
|---|---|
| **SID** | Server ID, 32 karakter hex, rahasia, identitas sekaligus kunci autentikasi agent |
| **Sample** | Satu kiriman agent (1 menit data) |
| **Raw** | Data per menit; retensi 14 hari |
| **Hourly** | Agregat per jam; retensi 365 hari |
| **Subject** | Objek spesifik sebuah alert (mount `/`, layanan `nginx`, sensor, target ping) |
| **Firing / Pending / Resolved** | Status event alert |
| **Slow table** | Tabel yang ditulis maksimal tiap 5 menit (disk, layanan, hardware) |
