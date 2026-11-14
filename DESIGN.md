# DESIGN.md — Rancangan Sistem LUMA Monitoring (v1.1)

> Tanggal: 2026-11-14. Menggantikan `rancangan-monitoring-server.md` v1.0.
> Detail kontrak ada di `PROTOCOL.md`, `DATABASE.md`, `API.md`. Dokumen ini fokus pada **perilaku dan arsitektur**.

## 1. Ringkasan dan Tujuan

Memantau ±60 server Linux: CPU, RAM, swap, disk, inode, I/O disk, jaringan, suhu, layanan, ping keluar, RAID/ZFS/drive. Dashboard modern, alert ke Telegram dan Email yang dikelola dari UI, deploy sederhana di Coolify.

**Bukan tujuan (v1):** monitoring website HTTP/SSL, multi-tenant, high availability, log aggregation, tracing.

## 2. Perubahan dari v1.0

| # | Perubahan | Alasan |
|---|---|---|
| 1 | Agent dikonfirmasi: **LUMA agent v2.4.2** (protokol identik dengan HetrixTools v2.4.2) | File agent sudah tersedia dan diperiksa |
| 2 | Endpoint agent: `POST /v2/` (data) dan `POST /` (event install/uninstall), bukan `/ingest` | Sesuai script, perubahan agent minimal |
| 3 | Agent **disajikan dari backend** (`/agent/*`) dengan placeholder `__LUMA_BASE_URL__` | Hilangkan ketergantungan GitHub, versi selaras dengan dashboard |
| 4 | Tabel `install_tokens` dihapus | Installer publik, kredensialnya adalah SID di argumen |
| 5 | Urutan primary key diubah menjadi `(server_id, ts, subject)` | Penghapusan retensi per server memakai indeks |
| 6 | Tambah tabel `server_events`, `ingest_rejections`, `zpool_status` | Timeline server dan diagnosis penolakan |
| 7 | `ts` = waktu terima di server (UTC), bukan field `time` dari agent | Field `time` berupa teks zona lokal |
| 8 | Status server disederhanakan; maintenance menjadi *overlay* (`maintenance_until`) | Menghindari status yang tumpang tindih |
| 9 | Prefiks env var `LUMA_` | Konsisten dan tidak bentrok |
| 10 | Keputusan terbuka v1.0 diberi nilai default (lihat `DECISIONS.md`) | Agar AI tidak menebak |
| 11 | Kasus tepi payload didokumentasikan (nilai `-`, suhu `0`, semua nilai JSON bertipe string) | Hasil audit kode agent |
| 12 | Set dokumen untuk vibe coding (`AGENT.md`, `TASKS.md`, dll.) | Pengerjaan terstruktur |

## 3. Arsitektur

```
[60 server]  luma_agent.sh (cron tiap menit)
     │  POST /v2/  (j=base64(gzip(JSON)))      POST /  (v=install|uninstall&s=SID)
     ▼
Proxy Coolify (HTTPS) ──► Container "luma" :8080
                            ├─ ingest handler ─► antrean (channel) ─► WRITER (1 goroutine) ─► SQLite (WAL)
                            ├─ /agent/*   (script agent hasil templating)
                            ├─ /api/*     (JSON, sesi cookie)
                            ├─ /healthz
                            ├─ /          (SPA React hasil embed)
                            └─ jobs: offline detector, alert evaluator, notification worker,
                                     agregasi jam, retensi, perawatan DB
                          Volume persisten /data/luma.db  ──► Litestream ──► S3-compatible
```

Prinsip:

1. **Satu penulis SQLite.** Semua penulisan lewat satu goroutine; pembaca memakai koneksi `mode=ro`.
2. **Ingest cepat dan murah.** Decode, validasi, antre, jawab `200`. Alert dan notifikasi asinkron.
3. **Agent tidak dipercaya.** Validasi ketat, batas ukuran, parser toleran.
4. **Secret tidak keluar dari backend.**
5. **Sederhana dulu.** Tanpa broker, cache eksternal, atau microservice.

## 4. Siklus Hidup Server

```
(dibuat di UI)            event v=install            data pertama
   pending  ───────────────► pending ─────────────────► online ◄──► offline
      ▲                                                    │
      │  rotate-sid / v=install ulang                      │ event v=uninstall
      └──────────────────────────────────────────────── decommissioned
```

| Status | Kondisi | Alert |
|---|---|---|
| `pending` | Server dibuat, belum pernah kirim data | Tidak ada |
| `online` | `now - last_seen ≤ LUMA_OFFLINE_AFTER` (default 180 dtk) | Normal |
| `offline` | `now - last_seen > LUMA_OFFLINE_AFTER` setelah pernah online | Aturan `offline` aktif |
| `decommissioned` | Event `v=uninstall` diterima | **Semua alert dinonaktifkan**, event terbuka di-resolve diam-diam |

- **Maintenance** adalah *overlay*: aktif jika `maintenance_until > now`. Selama aktif, evaluator melewati server itu dan notifikasi ditahan.
- Data masuk dari server `decommissioned` → status kembali `online` dan dicatat di `server_events`.
- Setiap perubahan status menulis baris `server_events`.

## 5. Alur Ingest (ringkas, detail di `PROTOCOL.md`)

1. `MaxBytesReader` (default 5 MiB) → `ParseForm` → ambil `j`.
2. base64 decode → gunzip dengan batas hasil (default 20 MiB) → `json.Unmarshal` ke `map[string]string`.
3. SHA-256(SID) → cari server. Tidak ada → `403`, catat ke `ingest_rejections`, **berhenti**.
4. Jalankan parser per field. Kegagalan satu field tidak membatalkan sample; hitung `parse_warnings`.
5. Kirim ke antrean writer (buffer 256). Penuh → `503`.
6. Balas `200 ok`.

Writer: satu transaksi per sample; upsert `metrics` dan tabel turunan; tabel **slow** (disk, layanan, RAID/ZFS/drive) ditulis maksimal tiap 5 menit per server; selalu perbarui `servers.last_seen`, `server_state.snapshot_json`, dan status.

## 6. Mesin Alert

### 6.1 Jenis aturan (`alert_rules.kind`)

| kind | Subjek | Kondisi breach |
|---|---|---|
| `threshold` | tergantung metrik | `value <op> threshold` |
| `offline` | - | `now - last_seen > threshold` detik (default 180) |
| `service_down` | nama layanan | `running = false` |
| `reboot_required` | - | `reqreboot = 1` |
| `raid_degraded` | `mount|device` | state RAID mengandung `degraded` atau `FAILED` |
| `drive_unhealthy` | device | status drive `fail` |
| `zpool_unhealthy` | pool | health ≠ `ONLINE` |

### 6.2 Metrik `threshold` (`alert_rules.metric`)

| metric | Sumber | Subjek | Satuan |
|---|---|---|---|
| `cpu` | `metrics.cpu` | - | % |
| `ram` | `metrics.ram` | - | % |
| `swap` | `metrics.swap` | - | % |
| `load_per_core` | `metrics.load1 / servers.cpu_logical` | - | rasio |
| `disk_pct` | `used / total × 100` dari snapshot | mount | % |
| `inode_pct` | `inode_used / inode_total × 100` | mount | % |
| `temp` | `temps.celsius` (snapshot terbaru) | sensor | °C |
| `ping_loss` | `ping_results.loss_pct` | `target_name` | % |
| `ping_rtt_ms` | `rtt_us / 1000` | `target_name` | ms |

Operator: `>`, `>=`, `<`, `<=`. Nilai harus **segar**: sample terakhir tidak lebih tua dari `LUMA_OFFLINE_AFTER`; jika lebih tua, metrik itu tidak dievaluasi (aturan `offline` yang menangani).

### 6.3 Cakupan dan prioritas

`scope_type` ∈ `all | group | server`. Untuk tiap server dan tiap kombinasi `(kind, metric, severity)`, aturan dengan cakupan **paling spesifik** menang (`server` > `group` > `all`). Aturan dengan severity berbeda (mis. 85% warning dan 95% critical) dievaluasi **bersamaan**.
`subject_exclude` (regex Go opsional) mengecualikan subjek, mis. `^/(boot|snap)`.

### 6.4 State machine

Kunci deduplikasi: `(rule_id, server_id, subject)`. Hanya satu event aktif (`pending`/`firing`) per kunci (dijamin *partial unique index*).

```
tidak ada event ──breach──► pending(started_at=now)
pending ──breach selama ≥ for_seconds──► firing(fired_at=now) ──► kirim notifikasi
pending ──tidak breach──► dihapus (tanpa notifikasi)
firing  ──breach & now-last_notified_at ≥ repeat_seconds──► kirim notifikasi ulang
firing  ──tidak breach──► resolved(resolved_at=now) ──► kirim pesan pemulihan bila notify_resolved
```

- Evaluator berjalan tiap **30 detik**, memakai **nilai terbaru** (bukan rata-rata).
- `for_seconds = 0` → langsung `firing`.
- Hysteresis (opsional, default mati): `recover_threshold` pada metrik persen; lihat `DATABASE.md`.
- Server `decommissioned`, `pending`, atau sedang maintenance: dilewati.
- Acknowledge (`acked_at`) menghentikan **pengulangan**, bukan pesan pemulihan.

### 6.5 Aturan bawaan (seed saat migrasi pertama)

| Nama | kind/metric | Ambang | for | Severity |
|---|---|---|---|---|
| Server offline | `offline` | 180 dtk | 0 | critical |
| CPU tinggi | `threshold/cpu` | > 90 | 600 | warning |
| RAM tinggi | `threshold/ram` | > 90 | 600 | warning |
| Disk hampir penuh | `threshold/disk_pct` | > 85 | 300 | warning |
| Disk kritis | `threshold/disk_pct` | > 95 | 300 | critical |
| Inode hampir habis | `threshold/inode_pct` | > 90 | 300 | warning |
| Layanan mati | `service_down` | - | 120 | critical |
| Suhu tinggi | `threshold/temp` | > 85 | 300 | warning |
| Packet loss | `threshold/ping_loss` | > 20 | 300 | warning |
| Perlu reboot | `reboot_required` | - | 86400 | info |
| RAID degraded | `raid_degraded` | - | 0 | critical |
| Drive bermasalah | `drive_unhealthy` | - | 0 | critical |

Semua `scope_type='all'`, `repeat_seconds=3600`, `notify_resolved=1`, tanpa kanal (admin memilih kanal dari UI). Disk dengan `subject_exclude` bawaan `^/(boot|snap|run)`.

## 7. Notifikasi

```
perubahan state alert ─► pembuat pesan ─► notification_queue ─► worker ─► Telegram / SMTP
```

- Antrean di database → tahan restart. Worker mengambil `status='queued' AND next_attempt_at ≤ now`.
- **Retry:** backoff 30 dtk, 2 mnt, 10 mnt, 30 mnt, 2 jam; maksimal 6 percobaan, lalu `dead` (tampil di UI dengan tombol "Kirim ulang").
- Error permanen (token tidak valid, chat tidak ditemukan, autentikasi SMTP gagal) → langsung `dead` dan kanal ditandai `last_error`.
- Pemilihan kanal: aturan → daftar kanal (`alert_rule_channels`) ∩ kanal `enabled` ∩ `severity ≥ channel.min_severity`.

### 7.1 Telegram

`POST https://api.telegram.org/bot<TOKEN>/sendMessage`, `parse_mode=HTML`, semua teks dinamis di-escape (`&`, `<`, `>`). Potong pesan > 4096 karakter. Tangani `429` dengan `retry_after`. Fitur UI: deteksi chat lewat `getUpdates`, pesan tes.

### 7.2 Email

SMTP (`wneessen/go-mail`), mode `starttls | tls | none`, multi-penerima, versi teks + HTML. Fitur UI: email tes.

### 7.3 Template pesan (Indonesia)

Firing:
```
🔴 [CRITICAL] {server} — {judul}
{detail}
Nilai: {value} (ambang {op} {threshold})
Mulai: {waktu} ({tz})
Buka: {base_url}/servers/{id}
```
Resolved:
```
✅ [PULIH] {server} — {judul}
Durasi gangguan: {durasi}
```
Grup offline:
```
🔴 [CRITICAL] {n} server offline bersamaan
{daftar nama, maksimal 20}
```
Ikon: 🔴 critical, 🟠 warning, 🔵 info, ✅ pulih.

### 7.4 Anti-banjir

| Mekanisme | Aturan |
|---|---|
| Durasi minimum | `for_seconds` per aturan |
| Deduplikasi | satu event aktif per kunci |
| Pengulangan | `repeat_seconds` (default 3600) |
| **Pengelompokan offline** | Jika ≥ `group_offline_threshold` (default 5) event `offline` menjadi `firing` dalam `group_offline_window_s` (default 120), kirim **satu** ringkasan, tahan pesan individual |
| Maintenance | server dilewati |
| Quiet hours | severity `info`/`warning` ditahan (masuk antrean, dikirim setelah selesai); `critical` selalu dikirim |
| Rate limit kanal | default 20 pesan/menit; kelebihan digabung menjadi satu pesan ringkasan |

## 8. Retensi dan Agregasi

| Data | Retensi | Catatan |
|---|---|---|
| Tabel raw (per menit) | 14 hari | `LUMA_RETENTION_RAW_DAYS` |
| Tabel `slow` | 14 hari | ditulis tiap 5 menit |
| `*_hourly` | 365 hari | `LUMA_RETENTION_HOURLY_DAYS` |
| `ingest_rejections`, `notification_queue` (sent/dead) | 7 / 30 hari | |
| `alert_events` resolved, `server_events` | 365 hari | |
| `audit_log` | 365 hari | |

Job:

| Job | Jadwal | Fungsi |
|---|---|---|
| Agregasi jam | tiap jam menit ke-5 | Hitung `metrics_hourly`, `net_hourly` untuk 3 jam terakhir (catch-up), `INSERT ... ON CONFLICT DO UPDATE` |
| Retensi | harian 03:30 UTC | Hapus **per server** dalam batch 10.000 baris dengan jeda 100 ms, hanya data yang sudah diagregasi |
| Perawatan | harian 04:00 UTC | `PRAGMA wal_checkpoint(TRUNCATE)`, `PRAGMA optimize`; `VACUUM` bulanan jika DB > 2 GB dan disk lega |
| Pembersihan | harian | sesi kedaluwarsa, antrean lama |

Pemilihan sumber query grafik: lihat `API.md` bagian 4.

## 9. Keamanan

| Area | Kontrol |
|---|---|
| Transport | HTTPS oleh proxy Coolify; `LUMA_BASE_URL` wajib `https://` di produksi |
| Ingest | batas body dan dekompresi, validasi SID dulu, rate limit per SID dan per IP, SID hash SHA-256 |
| Sesi | cookie `HttpOnly; Secure; SameSite=Strict`, token acak 32 byte, disimpan hash; TTL default 12 jam; pengecekan `Origin` pada metode non-GET |
| Password | argon2id, minimal 12 karakter; login dibatasi 5/menit/IP + penguncian 15 menit setelah 10 kegagalan berturut |
| Secret kanal | AES-256-GCM; kunci dari `LUMA_SECRET_KEY` (bukan dari DB); API mengembalikan `********` |
| Header | CSP, `X-Content-Type-Options`, `Referrer-Policy: same-origin`, `frame-ancestors 'none'` |
| Proxy | `X-Forwarded-For` dipercaya hanya dari `LUMA_TRUSTED_PROXIES` |
| Data sensitif agent | `rps2` (proses) dan `cust` disimpan hanya jika fitur aktif, hanya terlihat admin |
| Teks mentah (`raid`, `dh`, `zp`) | ditampilkan sebagai teks, tidak pernah HTML |
| Audit | login, perubahan kanal/aturan/server/pengguna, tes kirim; tanpa nilai secret |
| Distribusi agent | file di `/agent/*` publik namun tidak berisi secret; sediakan `SHA256SUMS` |

## 10. Observabilitas

- Log JSON (`slog`): `ts, level, msg, request_id`, tanpa secret.
- `/healthz`: cek DB (`SELECT 1`), umur job utama; `200` atau `503`.
- Metrik internal (log periodik tiap 5 menit, tanpa endpoint publik): sample/menit, parse warning, panjang antrean writer dan notifikasi, durasi transaksi, ukuran DB dan WAL.
- Pasang agent pada VPS monitor sendiri. Pantau `/healthz` dari layanan eksternal (mis. Uptime Kuma di tempat lain).
- Graceful shutdown: tangani `SIGTERM`, hentikan HTTP, kosongkan antrean writer, checkpoint, tutup DB.

## 11. Kapasitas

| Item | Perkiraan |
|---|---|
| Sample | 60/menit (≈1/detik) |
| Baris per sample | 15-30 |
| Total | ±1.500 baris/menit, ±2,2 juta/hari |
| Raw 14 hari | 2-4 GB |
| Hourly 1 tahun | 200-500 MB |
| Disk VPS minimal | 40 GB (DB ≤ 15 GB + WAL + ruang VACUUM) |
| RAM aplikasi | puluhan MB (Coolify + proxy ±0,5-1 GB; VPS 2 GB nyaman) |

## 12. Risiko dan Mitigasi

| Risiko | Mitigasi |
|---|---|
| DB membesar lebih cepat | retensi ketat, alert disk untuk VPS monitor, ukur sejak fase 1 |
| Kehilangan volume `/data` | Litestream ke luar VPS; uji restore (T-093) |
| Kehilangan `LUMA_SECRET_KEY` | simpan di password manager; kanal harus dikonfigurasi ulang |
| Perubahan format agent | parser toleran, fixture nyata, agent disajikan dari repo sendiri (versi terkunci) |
| Banjir notifikasi | pengelompokan, rate limit, cooldown |
| Monitor mati tanpa terdeteksi | pemantauan eksternal `/healthz` |
| Lisensi script agent | verifikasi lisensi asal (T-001), pertahankan atribusi |
| Satu penulis jadi bottleneck | beban ≈ 1 tx/detik, jauh di bawah kapasitas; uji beban T-102 |
| Satuan field berbeda | normalisasi di parser, diuji dengan fixture |
