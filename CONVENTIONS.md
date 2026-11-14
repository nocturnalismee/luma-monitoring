# CONVENTIONS.md — Konvensi Kode, Pengujian, dan Dependensi

## 1. Umum

- Bahasa: kode, komentar, nama identifier, pesan commit, nama field API = **Inggris**. Teks UI dan pesan notifikasi = **Indonesia**.
- Satu fungsi = satu tanggung jawab; fungsi > 60 baris dipecah.
- Komentar menjelaskan **mengapa**, bukan **apa**. Setiap fungsi publik punya doc comment.
- Tidak ada kode mati, kode yang dikomentari, atau `TODO` tanpa `(T-xxx)`.
- Penanda asumsi AI: `// ASSUMPTION(T-xxx): ...` (harus dibahas dengan pengguna di ringkasan).

## 2. Git

- Cabang: `feat/T-014-writer`, `fix/...`, `docs/...`.
- Commit (Conventional Commits): `feat(ingest): ...`, `fix(parser): ...`, `test(...)`, `docs(...)`, `chore(...)`, `refactor(...)`.
- Satu tugas = satu PR kecil (< ~500 baris diff di luar fixture dan lockfile).
- Dilarang commit: `*.db`, `.env`, SID/token asli, payload tanpa anonimisasi.

## 3. Go

### 3.1 Gaya

- `gofmt`, `go vet`, dan `staticcheck` (jika tersedia) harus bersih.
- Nama paket pendek, huruf kecil, tanpa underscore. Tidak ada paket `util`/`common`/`helpers`.
- `context.Context` argumen pertama untuk fungsi yang melakukan I/O atau bisa lama.
- Tidak ada variabel global yang bisa berubah. Dependensi lewat konstruktor (`NewX(deps)`). Antarmuka didefinisikan **di sisi pemakai**, kecil (1-3 metode).
- Waktu: gunakan `type Clock interface{ Now() time.Time }`; produksi memakai `time.Now`, tes memakai jam palsu. Dilarang `time.Now()` langsung di logika alert/job/parser.
- Konkurensi: setiap goroutine punya pemilik, berhenti lewat `ctx.Done()`, dan dicakup `errgroup`/`WaitGroup` saat shutdown. Dilarang goroutine "fire and forget".

### 3.2 Error

- Bungkus: `fmt.Errorf("write sample: %w", err)`. Error sentinel untuk kasus yang diperiksa pemanggil (`ErrUnknownSID`, `ErrPayloadTooLarge`, `ErrNotFound`).
- Jangan `panic` kecuali kesalahan pemrograman saat startup. Handler HTTP dilindungi middleware `recover`.
- Jangan mengembalikan error internal ke klien; petakan ke kode API (`API.md` bagian 1).

### 3.3 Logging (`log/slog`)

- Bidang terstruktur: `slog.Info("sample written", "server_id", id, "rows", n, "dur_ms", d)`.
- **Dilarang** mencatat: SID, token, password, isi payload mentah, header `Cookie`, body login.
- Level: `debug` detail parsing, `info` peristiwa penting (start, status server berubah, job selesai), `warn` data aneh/parse warning, `error` kegagalan yang butuh perhatian.

### 3.4 HTTP

- `ServeMux` Go 1.22: `mux.HandleFunc("GET /api/servers/{id}", h)`. Ambil parameter dengan `r.PathValue("id")`.
- Middleware (urut): request ID → recover → logging → security headers → rate limit → autentikasi → otorisasi peran.
- Handler: decode → validasi → panggil service → encode. Tanpa SQL di handler.
- Body JSON dibatasi (`http.MaxBytesReader`, 1 MiB untuk API biasa), `DisallowUnknownFields` pada decoder.
- Respons JSON lewat helper `writeJSON(w, status, v)`; error lewat `writeError(w, status, code, msg, fields)`.
- Semua `http.Server` punya `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`.

### 3.5 SQL (paket `store`)

- Parameter `?`. Tanpa ORM. Sebut kolom eksplisit.
- Query baca → `reader` DB; tulis → lewat `writer.Do`.
- Transaksi singkat; tidak ada I/O jaringan di dalam transaksi.
- Pemindaian `NULL` memakai `sql.NullFloat64`/`*float64` sesuai kolom; konversi di lapisan store.
- Uji store dengan SQLite di memori/berkas sementara (`t.TempDir()`) dan migrasi asli.

### 3.6 Parser (`internal/parser`)

- Fungsi murni: `ParseDisks(s string) ([]Disk, []Warning)`. Tidak melakukan I/O, tidak membaca jam, tidak mengakses DB.
- Tidak pernah `panic` pada input apa pun (uji dengan `go test -fuzz` untuk parser daftar).
- Tidak gagal karena satu baris buruk: lewati dan kembalikan `Warning{Field, Reason}`.

## 4. TypeScript / React

- `tsconfig`: `strict`, `noUncheckedIndexedAccess`, `noImplicitOverride`. Dilarang `any` dan `// @ts-ignore` (gunakan `@ts-expect-error` dengan alasan).
- Komponen fungsi + hooks. Satu komponen per file, nama `PascalCase.tsx`; hook `useXxx.ts`.
- Data server hanya via TanStack Query; state UI lokal via `useState`, lintas halaman via Zustand.
- Form: react-hook-form + zod resolver; pesan error zod dalam Indonesia.
- Styling: kelas Tailwind; hindari CSS kustom kecuali token tema. Warna lewat variabel CSS tema, bukan hex di komponen.
- Aksesibilitas: elemen interaktif dapat difokus, label pada input, `aria-label` pada tombol ikon.
- ESLint + Prettier wajib bersih; impor diurutkan otomatis.

## 5. Pengujian

| Tingkat | Alat | Wajib untuk |
|---|---|---|
| Unit Go | `testing` + table-driven | parser, state machine alert, kriptografi, downsampling, pemilihan `step`/sumber |
| Golden/fixture | `testdata/payloads/*.body` + `*.golden.json` | pipeline decode → parse (T-012) |
| Integrasi Go | `httptest` + SQLite sementara | `/v2/`, event, auth, API read, job agregasi |
| Fuzz | `go test -fuzz` | parser daftar (`disks`, `oping`, `raid`) |
| Race | `go test -race ./...` | selalu di CI |
| Unit FE | Vitest + Testing Library | `format.ts`, `mergeSeries`, `thresholds`, komponen inti |
| E2E | Playwright (fase 10) | login, tambah server, buat aturan, tes kanal |

Aturan:
- Setiap bug diperbaiki **dengan tes regresi**.
- Tes deterministik: tanpa `sleep`, tanpa jaringan nyata (Telegram/SMTP dipalsukan lewat `httptest`/server SMTP palsu), jam palsu.
- Cakupan target: parser dan alert ≥ 85%, paket lain ≥ 60% (indikator, bukan tujuan).

## 6. Keamanan (checklist peninjauan)

- [ ] Tidak ada SQL dibangun dari input luar.
- [ ] Tidak ada secret di log, respons, fixture, atau error.
- [ ] Batas ukuran/waktu pada semua input luar.
- [ ] Peran dicek di sisi server untuk setiap endpoint admin.
- [ ] Perbandingan token/hash memakai `subtle.ConstantTimeCompare`.
- [ ] Teks dinamis di-escape pada Telegram HTML dan email HTML.
- [ ] Cookie `HttpOnly`, `Secure`, `SameSite=Strict`.
- [ ] Respons error tidak membocorkan keberadaan akun/SID (pesan generik).

## 7. Allowlist Dependensi

Menambah dependensi di luar daftar ini **membutuhkan entri ADR baru** di `DECISIONS.md` (alasan, alternatif stdlib, ukuran, pemeliharaan, lisensi).

**Go**

| Paket | Fungsi |
|---|---|
| `modernc.org/sqlite` | Driver SQLite |
| `golang.org/x/crypto` | `argon2` |
| `golang.org/x/time` | `rate` (rate limiter) |
| `github.com/wneessen/go-mail` | Email SMTP |
| `github.com/pquerna/otp` | TOTP (fase 10) |

Selain itu hanya stdlib (`net/http`, `database/sql`, `encoding/json`, `compress/gzip`, `crypto/*`, `log/slog`, `embed`, `testing`, ...).

**Frontend (runtime)**: `react`, `react-dom`, `react-router-dom`, `@tanstack/react-query`, `@tanstack/react-table`, `zustand`, `react-hook-form`, `@hookform/resolvers`, `zod`, `recharts`, `lucide-react`, `cmdk`, `clsx`, `tailwind-merge`, `class-variance-authority`, paket `@radix-ui/*` yang ditarik shadcn/ui, `sonner`.

**Frontend (dev)**: `vite`, `@vitejs/plugin-react`, `typescript`, `tailwindcss`, `postcss`, `autoprefixer`, `eslint` + plugin React/TS, `prettier`, `vitest`, `@testing-library/*`, `jsdom`, `msw`, `@playwright/test`.

## 7a. Versi

- Pin versi mayor di `go.mod` dan `package.json` (lockfile di-commit). Jangan memakai `latest`.
- Pilih rilis paket yang sudah beredar minimal 2 minggu saat menambah dependensi.

## 8. Definition of Done

Lihat `AGENT.md` bagian 11. Ringkas: lint dan tes hijau, tes jalur normal + dua kasus tepi, tanpa secret bocor, dokumen terkini, ringkasan ditulis.
