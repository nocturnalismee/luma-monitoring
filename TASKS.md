# TASKS.md — Daftar Tugas Berurutan

> Cara pakai: kerjakan dari atas ke bawah. Satu tugas pada satu waktu. Centang `[x]` hanya setelah **Definition of Done** (`AGENT.md` bagian 11) terpenuhi.
> Format: **ID — Judul** · *Prasyarat* · **Refs** (wajib dibaca) · **Buat** · **Terima** (kriteria penerimaan yang dapat diverifikasi).

## Fase 0 — Fondasi

- [ ] **T-000 — Bootstrap repo** · *tanpa*
  - Refs: `AGENT.md` 5-6, `CONVENTIONS.md`
  - Buat: struktur folder, `backend` (`go mod init github.com/nocturnalismee/luma-monitoring/backend`), `frontend` (Vite React TS), `Makefile` (target di `AGENT.md` bagian 6, termasuk `sync-agent`), `.gitignore` (termasuk `*.db`, `.env`, `backend/internal/agentdist/files/*` kecuali `README.txt`), `.editorconfig`, `.env.example`, CI GitHub Actions (lint + test backend dan frontend).
  - Terima: `make lint` dan `make test` berjalan hijau pada repo kosong; CI hijau.

- [x] **T-001 — Impor dan siapkan agent** · *T-000*
  - Refs: `DEPLOYMENT.md` 4.2, `DECISIONS.md` Q-001, `PROTOCOL.md` 6
  - Buat: salin kelima file agent ke `agent/`, jalankan perintah `sed` di `DEPLOYMENT.md` 4.2, tambah `agent/README.md` (asal file, perubahan yang dilakukan, versi 2.4.2).
  - Terima: `grep` tidak menemukan `sm.luma.local` maupun `raw.githubusercontent`; `bash -n` lolos untuk semua `.sh`; `git diff` terhadap file asli hanya berisi penggantian URL; pertanyaan Q-001 dicatat ke pengguna.
  - Catatan penyelesaian (2026-11-14): kelima file sudah berada di `agent/` dari impor awal; `sed` bagian 4.2 dijalankan atas 9 literal URL (2 di `luma_agent.sh`, 4 di `luma_install.sh`, 2 di `luma_update.sh`, 1 di `luma_uninstall.sh`); `agent/README.md` diberi bagian "Provenance and repository notes". Prasyarat T-000 belum tercentang dan `make lint` / `make test` belum tersedia (`AGENT.md` 6) — lihat DECISIONS.md Q-008.

- [ ] **T-002 — Konfigurasi, logging, server dasar** · *T-000*
  - Refs: `DEPLOYMENT.md` 1, `CONVENTIONS.md` 3
  - Buat: `internal/config` (env → struct, validasi, error jelas), `slog` JSON, `cmd/luma/main.go` dengan graceful shutdown (`SIGTERM`), `GET /healthz`, middleware request ID + recover + logging.
  - Terima: start gagal dengan pesan jelas bila `LUMA_BASE_URL`/`LUMA_SECRET_KEY` hilang atau invalid (tanpa mencetak nilai); tes konfigurasi table-driven; `/healthz` → `200`.

- [ ] **T-003 — Database dan migrasi** · *T-002*
  - Refs: `DATABASE.md` 1-3
  - Buat: `internal/db` (writer 1 koneksi, reader read-only, pragma via DSN), runner migrasi ter-*embed* (`schema_migrations`), `0001_init.sql` (salin skema bagian 3), `writer.Do(fn)`.
  - Terima: migrasi berjalan idempoten; tes memastikan `PRAGMA journal_mode` = `wal` dan `foreign_keys` = 1; tes memastikan pembaca tidak bisa menulis; tes dua `writer.Do` bersamaan berjalan berurutan tanpa `database is locked`.

- [ ] **T-004 — Dockerfile dan deploy kosong** · *T-003*
  - Refs: `DEPLOYMENT.md` 2-3, 7, `FRONTEND.md` 9
  - Buat: `Dockerfile`, `.dockerignore`, placeholder `backend/internal/web/dist/index.html`, embed + fallback SPA, `internal/agentdist/files/README.txt`.
  - Terima: `make docker` sukses; container berjalan dengan volume `/data`; `/healthz` `200`; data tetap ada setelah container dibuat ulang; (opsional) deploy di Coolify dan akses via HTTPS.

## Fase 1 — Ingest

- [ ] **T-010 — Decode amplop payload** · *T-003*
  - Refs: `PROTOCOL.md` 2
  - Buat: `internal/ingest/decode.go`: form → base64 → gunzip dengan batas → `map[string]string`; error bertipe (`ErrTooLarge`, `ErrBadPayload`).
  - Terima: tes untuk payload valid, base64 rusak, gzip rusak, gzip bomb (melebihi batas → `ErrTooLarge`), field `j` hilang, `%2F`/`%2B` ter-decode benar.

- [ ] **T-011a — Parser skalar dan teks** · *T-010*
  - Refs: `PROTOCOL.md` 3, 4.1
  - Buat: `internal/parser/meta.go`: konversi angka toleran (string non-numerik → `NULL`), clamp persen, field teks base64 (`os`, `kernel`, `hostname`, `cpumodel`), struct `Sample`.
  - Terima: tes table-driven termasuk nilai kosong, `nan`, persen > 100, base64 tidak valid.

- [ ] **T-011b — Parser daftar** · *T-011a*
  - Refs: `PROTOCOL.md` 4.2, 4.3
  - Buat: `disks`, `inodes` (`-` → NULL), `iops`, `nics`, `ipv4/ipv6`, `conn`, `temp` (÷1000, abaikan `0`, rentang valid), `serv`, `oping` (deteksi `proto`, normalisasi `rtt_us`).
  - Terima: tes tiap fungsi (normal, entri kosong, kolom kurang/lebih, duplikat mount); `oping` ICMP vs TCP menghasilkan `rtt_us` benar; fuzz test tidak pernah panic.

- [ ] **T-011c — Parser hardware** · *T-011a*
  - Refs: `PROTOCOL.md` 4.4
  - Buat: `raid` (`SplitN` 3, deteksi degraded), `zp` (base64 bersarang, health), `dh` (tipe 1/2/3, turunan status), `cust`.
  - Terima: tes dengan contoh keluaran `mdadm -D`, `zpool status`, `smartctl`, `nvme smart-log` (sintetis) — status ok/fail/unknown benar.

- [ ] **T-012 — Fixture nyata dan golden test** · *T-011b, T-011c*
  - Refs: `PROTOCOL.md` 9
  - Buat: minimal 3 fixture nyata (anonim) di `testdata/payloads/`, `*.golden.json`, target `make replay`.
  - Terima: tes golden hijau; tidak ada SID/hostname/IP asli di repo (`grep`); pengguna diminta menyediakan fixture nyata bila belum ada (jangan mengarang fixture "nyata").

- [ ] **T-013 — Autentikasi SID dan rate limit** · *T-003*
  - Refs: `DESIGN.md` 5, 9; `API.md` 10; `DATABASE.md` (`ingest_rejections`)
  - Buat: hash SHA-256, lookup (cache memori dengan invalidasi), `subtle.ConstantTimeCompare`, rate limiter per SID dan per IP (`x/time/rate`), pencatat `ingest_rejections` (maks 1 baris/IP/menit per alasan), ekstraksi IP klien memakai `LUMA_TRUSTED_PROXIES`.
  - Terima: SID tidak dikenal → ditolak tanpa parsing; tes IP klien dengan/ tanpa proxy tepercaya; SID tidak muncul di log.

- [ ] **T-014 — Writer dan penyimpanan sample** · *T-011b, T-011c, T-013*
  - Refs: `DESIGN.md` 5, `DATABASE.md` 3-5, 7
  - Buat: `internal/writer` (channel 256, satu goroutine), `store.WriteSample` (satu transaksi: `metrics`, `net_usage`, `disk_io`, `temps`, `ping_results`, `conn_counts`; tabel slow tiap 5 menit; `server_state`; hardware upsert; perbarui `servers` termasuk `first_seen`, `last_seen`, metadata, `server_ips`), idempotensi menit yang sama.
  - Terima: tes integrasi mengirim sample dua kali pada menit sama → satu baris; tabel slow tidak bertambah dalam 5 menit; snapshot sesuai struktur `DATABASE.md` 4; antrean penuh → fungsi enqueue mengembalikan error (dipetakan `503`).

- [ ] **T-015 — Handler `POST /v2/` dan `POST /`** · *T-014*
  - Refs: `PROTOCOL.md` 1, 7; `API.md` 2
  - Buat: handler ingest (urutan `DESIGN.md` 5), handler event install/uninstall, status server (`pending→online`, `decommissioned`, kembali `online`), `server_events`.
  - Terima: replay fixture → `200 ok` dan data tersimpan; `v=uninstall` → `decommissioned` dan event alert terbuka di-resolve; SID salah → `403`; body > batas → `413`.

- [ ] **T-016 — Offline detector** · *T-015*
  - Refs: `DESIGN.md` 4, 8
  - Buat: job tiap 30 dtk memakai `Clock`: `online→offline` bila `now-last_seen > LUMA_OFFLINE_AFTER`; menulis `server_events`.
  - Terima: tes dengan jam palsu; server `pending` dan `decommissioned` tidak pernah menjadi `offline`.

## Fase 2 — Autentikasi Admin

- [ ] **T-020 — Pengguna dan bootstrap admin** · *T-003*
  - Refs: `DESIGN.md` 9, `API.md` 3, 9
  - Buat: argon2id, `store.Users`, bootstrap dari `LUMA_ADMIN_*` bila tabel kosong, validasi password ≥ 12.
  - Terima: tes hash/verify; bootstrap hanya berjalan sekali; password tidak pernah dilog.

- [ ] **T-021 — Sesi dan otorisasi** · *T-020*
  - Refs: `DESIGN.md` 9, `API.md` 1, 3
  - Buat: login/logout/`me`, cookie sesi (`HttpOnly; Secure; SameSite=Strict`), token acak disimpan hash, pengecekan `Origin`, middleware peran `viewer`/`admin`.
  - Terima: tanpa sesi `401`; viewer ke rute admin `403`; Origin salah `403`; logout mencabut sesi; tes untuk sesi kedaluwarsa.

- [ ] **T-022 — Perlindungan login** · *T-021*
  - Refs: `DESIGN.md` 9
  - Buat: rate limit 5/menit/IP, penguncian akun (10 gagal → 15 menit), pesan error generik.
  - Terima: tes urutan kegagalan → terkunci → terbuka setelah waktu; respons tidak membedakan "email tidak ada" vs "password salah".

- [ ] **T-023 — Audit log** · *T-021*
  - Refs: `DATABASE.md` (`audit_log`), `API.md` 9
  - Buat: helper `audit.Record(ctx, action, target, detail)`, `GET /api/audit-log`, dipakai oleh login/logout.
  - Terima: detail tidak pernah berisi secret (tes dengan nilai pancingan).

## Fase 3 — API dan UI Inti

- [ ] **T-030 — Query baca: overview dan daftar server** · *T-014*
  - Refs: `API.md` 4.1-4.3, `DATABASE.md` 5
  - Buat: `store` read queries, termasuk `spark` (12 titik/5 menit) dan `disk_max_pct` dari snapshot.
  - Terima: tes dengan data sintetis; satu query agregat untuk spark seluruh server (tanpa N+1).

- [ ] **T-031 — Handler overview/servers/detail/groups** · *T-021, T-030*
  - Refs: `API.md` 4.1-4.3, 4.7
  - Terima: bentuk JSON persis `API.md`; filter `status/group_id/q/sort/limit/offset`; `agent_latest_version` terisi.

- [ ] **T-032 — Endpoint deret waktu `metrics`** · *T-031*
  - Refs: `API.md` 4.4, `DESIGN.md` 8
  - Buat: pemilihan `step` (≤ ~500 titik) dan `source`, downsampling SQL, sumber `hourly` untuk > 14 hari.
  - Terima: tes tabel-driven untuk pemilihan `step`/`source` di berbagai rentang; titik tidak melebihi ~500; bucket kosong tidak menghasilkan titik palsu.

- [ ] **T-033 — Scaffold frontend** · *T-021*
  - Refs: `FRONTEND.md` 1-3, 9; `CONVENTIONS.md` 4
  - Buat: Tailwind, shadcn init + komponen, React Router, TanStack Query, `lib/api.ts`, guard auth, `AppShell`, tema dark/light, `lib/format.ts` + tes.
  - Terima: `npm run lint`, `typecheck`, `test`, `build` hijau; navigasi berfungsi dengan data tiruan.

- [ ] **T-034 — Halaman Login** · *T-033*
  - Refs: `FRONTEND.md` 2, `API.md` 3
  - Terima: login berhasil → `next`; gagal menampilkan pesan; sesi habis → kembali ke login.

- [ ] **T-035 — Halaman Overview** · *T-031, T-033*
  - Refs: `FRONTEND.md` 4.1, 7
  - Terima: KPI, tabel, filter, sparkline, badge status, skeleton/empty/error state; polling 15 dtk.

- [ ] **T-036 — Detail server: tab Ringkasan** · *T-032, T-035*
  - Refs: `FRONTEND.md` 4.2, 5
  - Buat: `TimeSeriesChart`, `RangeSelector`, grafik CPU/Load/RAM/Swap, crosshair sinkron.
  - Terima: pergantian rentang memuat ulang data; nilai `null` memutus garis; tooltip memakai `display_tz`.

- [ ] **T-037 — Embed frontend dan SPA fallback** · *T-004, T-033*
  - Refs: `FRONTEND.md` 9
  - Terima: binary tunggal melayani UI; refresh di `/servers/12` tidak 404; `/api/*` tidak pernah jatuh ke `index.html`; header cache sesuai.

## Fase 4 — Grafik dan Detail Lengkap

- [ ] **T-040 — Endpoint network, disk-io, temps, ping, connections** · *T-032*
  - Refs: `API.md` 4.4
  - Terima: bentuk seri berkunci persis; `rtt_ms = rtt_us/1000`; rentang > 14 hari → `source:"none"` untuk endpoint raw-only.

- [ ] **T-041 — Tab Jaringan & Disk (grafik)** · *T-040, T-036*
  - Refs: `FRONTEND.md` 4.2, 5, 6
  - Terima: bit/detik di jaringan, byte/detik di disk; `mergeSeries` teruji.

- [ ] **T-042 — Tab Layanan & Perangkat, endpoint state/events** · *T-041*
  - Refs: `API.md` 4.5-4.6, `FRONTEND.md` 4.2
  - Terima: tabel disk dengan `UsageBar` dan inode; layanan; ping; RAID/ZFS/drive; teks mentah hanya untuk admin dan ditampilkan di `<pre>` (bukan HTML).

## Fase 5 — Manajemen Server

- [ ] **T-050 — CRUD server, grup, rotate SID, maintenance** · *T-031*
  - Refs: `API.md` 5, `DESIGN.md` 4, `AGENT.md` 9
  - Terima: SID hanya di respons pembuatan/rotasi; DB hanya menyimpan hash; rotate → SID lama ditolak dan status `pending`; maintenance menulis `server_events`; semua aksi tercatat di audit.

- [ ] **T-051 — Distribusi agent `/agent/*`** · *T-001, T-004*
  - Refs: `DEPLOYMENT.md` 4
  - Buat: `internal/agentdist` (embed, templating sekali saat start, allowlist, `SHA256SUMS`, `HEAD`), pembacaan `Version=`.
  - Terima: respons tidak mengandung `__LUMA_BASE_URL__`; nama di luar allowlist `404`; `SHA256SUMS` cocok dengan `sha256sum` atas respons; `wget --spider` berhasil.

- [ ] **T-052 — UI Kelola Server** · *T-050, T-051, T-033*
  - Refs: `FRONTEND.md` 4.4
  - Terima: dialog menampilkan SID sekali + perintah install; salin ke clipboard; peringatan root; konfirmasi hapus.

- [ ] **T-053 — Uji instalasi agent end-to-end** · *T-015, T-051*
  - Buat: panduan manual + skrip uji di VM/container Linux (instal, data masuk, update, uninstall).
  - Terima: server uji `pending → online`; `luma_update.sh` melaporkan "already at latest"; uninstall → `decommissioned` dan tanpa alert offline. Hasil dilaporkan ke pengguna.

## Fase 6 — Alert

- [ ] **T-060 — Aturan: CRUD dan seed** · *T-021*
  - Refs: `DESIGN.md` 6, `DATABASE.md` (`alert_rules`), `API.md` 6.2
  - Buat: `0002_seed_rules.sql` (tabel `DESIGN.md` 6.5), endpoint CRUD + validasi.
  - Terima: validasi kombinasi kind/metric/op; regex `subject_exclude` tidak valid → `400`.

- [ ] **T-061 — Evaluator threshold** · *T-060, T-014*
  - Refs: `DESIGN.md` 6.2-6.4
  - Buat: evaluator 30 dtk, prioritas cakupan, subjek, state machine, dedup (unique index), hysteresis `recover_threshold`, `Clock`.
  - Terima: tes state machine lengkap (pending→dibatalkan, pending→firing, repeat, resolved, ack menghentikan repeat, server maintenance/decommissioned dilewati, data usang tidak dievaluasi).

- [ ] **T-062 — Evaluator non-threshold** · *T-061, T-016*
  - Refs: `DESIGN.md` 6.1
  - Buat: `offline`, `service_down`, `reboot_required`, `raid_degraded`, `drive_unhealthy`, `zpool_unhealthy`.
  - Terima: tiap kind punya tes; `v=uninstall` meresolve diam-diam.

- [ ] **T-063 — API dan UI Alerts** · *T-062, T-035*
  - Refs: `API.md` 6.1, `FRONTEND.md` 4.3
  - Terima: filter; ack hanya admin; badge jumlah alert di sidebar.

## Fase 7 — Notifikasi

- [ ] **T-070 — Kripto AES-GCM** · *T-002*
  - Refs: `DESIGN.md` 9, `DEPLOYMENT.md` 1
  - Terima: enkripsi/dekripsi round-trip; nonce unik; data diubah → gagal; kunci salah → gagal; tes tidak mencetak kunci.

- [ ] **T-071 — Kanal: CRUD dengan penyamaran secret** · *T-070, T-021*
  - Refs: `API.md` 7
  - Terima: respons selalu `********`; PUT dengan `********`/tanpa field mempertahankan nilai lama; validasi per tipe.

- [ ] **T-072 — Antrean dan worker** · *T-071*
  - Refs: `DESIGN.md` 7, `DATABASE.md` (`notification_queue`)
  - Terima: retry backoff sesuai jadwal (jam palsu); `dead` setelah 6 percobaan; error permanen langsung `dead`; tahan restart (tes buka-tutup DB).

- [ ] **T-073 — Telegram** · *T-072*
  - Refs: `DESIGN.md` 7.1, `API.md` 7
  - Terima: dengan `httptest` — escape HTML, pemotongan 4096, penanganan `429 retry_after`, `detect-chats`, tes kirim.

- [ ] **T-074 — Email SMTP** · *T-072*
  - Refs: `DESIGN.md` 7.2
  - Terima: dengan server SMTP palsu — `starttls|tls|none`, multi-penerima, teks + HTML, escape.

- [ ] **T-075 — Pembuat pesan dan template** · *T-072, T-061*
  - Refs: `DESIGN.md` 7.3
  - Terima: pesan firing/resolved/grup sesuai template; nama server berisi `<script>` aman.

- [ ] **T-076 — UI Kanal** · *T-071, T-073, T-074*
  - Refs: `FRONTEND.md` 4.5
  - Terima: form Telegram (deteksi chat, tes), form Email (tes), toggle aktif, field secret bertopeng.

- [ ] **T-077 — UI Aturan** · *T-060, T-076*
  - Terima: form validasi zod, pilihan kanal, pratinjau kalimat aturan.

- [ ] **T-078 — Log notifikasi dan retry** · *T-072*
  - Refs: `API.md` 8
  - Terima: daftar + filter; retry hanya untuk `dead`; UI menampilkan error tersanitasi.

## Fase 8 — Anti-banjir

- [ ] **T-080 — Pengelompokan offline** · *T-075*
  - Refs: `DESIGN.md` 7.4
  - Terima: 6 server offline dalam 120 dtk → 1 pesan grup; 2 server → pesan individual.

- [ ] **T-081 — Quiet hours** · *T-075*
  - Terima: `critical` selalu dikirim; `warning/info` ditahan dan dikirim setelah selesai; menangani rentang melewati tengah malam dan zona waktu.

- [ ] **T-082 — Repeat dan cooldown** · *T-061*
  - Terima: pengulangan tepat tiap `repeat_seconds`; ack menghentikan.

- [ ] **T-083 — Rate limit per kanal** · *T-072*
  - Terima: >20 pesan/menit digabung menjadi ringkasan; tes dengan jam palsu.

## Fase 9 — Retensi dan Backup

- [ ] **T-090 — Agregasi per jam** · *T-014*
  - Refs: `DATABASE.md` 5, `DESIGN.md` 8
  - Terima: idempoten; catch-up 3 jam; nilai benar terhadap data sintetis.

- [ ] **T-091 — Retensi** · *T-090*
  - Terima: hanya menghapus data lebih tua dari retensi **dan** sudah diagregasi; batch 10.000; tidak memblokir writer terlalu lama (uji ukuran batch).

- [ ] **T-092 — Perawatan DB** · *T-091*
  - Terima: checkpoint/optimize terjadwal; VACUUM bersyarat; dicatat di log.

- [ ] **T-093 — Litestream dan uji restore** · *T-004*
  - Refs: `DEPLOYMENT.md` 6, `DECISIONS.md` Q-002
  - Terima: replikasi berjalan; **restore berhasil** pada instance bersih; prosedur terdokumentasi dan dilaporkan.

- [ ] **T-094 — Pemilihan sumber hourly di query** · *T-090, T-032*
  - Terima: rentang > 14 hari menggunakan `*_hourly` dan tetap ≤ ~500 titik.

## Fase 10 — Hardening

- [ ] **T-100 — TOTP (2FA)** · *T-021, T-070*
  - Terima: aktivasi dengan verifikasi kode; login meminta `totp`; secret terenkripsi; kode pemulihan sekali pakai.

- [ ] **T-101 — Header keamanan dan CSP** · *T-037*
  - Refs: `DESIGN.md` 9
  - Terima: CSP tidak memblokir UI; header hadir di semua respons; tes otomatis.

- [ ] **T-102 — Uji beban** · *T-015*
  - Buat: `scripts/loadtest` mensimulasikan 60 dan 200 agent (payload fixture, SID berbeda).
  - Terima: latensi ingest p95 < 200 ms; ukuran DB dan waktu job agregasi/retensi dilaporkan.

- [ ] **T-103 — E2E dan runbook** · *semua*
  - Terima: Playwright untuk alur kritis; `docs/RUNBOOK.md` (upgrade, rollback, restore, rotate SID, insiden umum).

- [ ] **T-104 — Tinjauan akhir** · *T-103*
  - Terima: checklist `CONVENTIONS.md` 6 lolos; `DECISIONS.md` tidak punya Q terbuka yang memblokir; versi dokumen diperbarui.
