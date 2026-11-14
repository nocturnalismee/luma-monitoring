# DECISIONS.md — Keputusan Arsitektur dan Pertanyaan Terbuka

> Status: **Terkunci** (jangan diubah tanpa ADR baru), **Default** (nilai awal yang dipilih; pengguna boleh mengganti), **Terbuka** (butuh jawaban pengguna).
> Menambah ADR: tambah baris baru di akhir tabel, jangan menghapus yang lama (beri status `Digantikan oleh ADR-xxx`).

## 1. Keputusan

| ID | Keputusan | Status | Alasan singkat |
|---|---|---|---|
| ADR-001 | Backend Go, satu binary, `net/http` stdlib (ServeMux Go 1.22) | Terkunci | RAM kecil, deploy sederhana, tanpa framework |
| ADR-002 | Database SQLite (WAL) via `modernc.org/sqlite`, satu penulis | Terkunci | 60 server ≈ 1 tx/detik; tanpa proses DB terpisah |
| ADR-003 | Frontend React + Vite + TS + Tailwind + shadcn/ui + TanStack Query + Recharts | Terkunci | Ekosistem terbesar, cocok untuk AI coding |
| ADR-004 | Routing frontend: React Router 6 (bukan TanStack Router) | Terkunci | Stabil, paling dikenal |
| ADR-005 | Agent = LUMA v2.4.2; endpoint `POST /v2/` dan `POST /` dipertahankan | Terkunci | Perubahan agent minimal |
| ADR-006 | Agent disajikan dari backend (`/agent/*`) via templating `__LUMA_BASE_URL__` | Terkunci | Tanpa GitHub, versi selaras, mengurangi risiko rantai pasokan |
| ADR-007 | `ts` = waktu terima server dibulatkan ke menit (UTC) | Terkunci | Field `time` agent berupa teks zona lokal |
| ADR-008 | SID = 16 byte acak → 32 hex; disimpan SHA-256 | Terkunci | Panjang 32 diwajibkan installer; hash mencegah kebocoran dari DB |
| ADR-009 | Sesi cookie + tabel `sessions` (bukan JWT) | Terkunci | Dapat dicabut, lebih sederhana |
| ADR-010 | Secret kanal dienkripsi AES-256-GCM, kunci dari `LUMA_SECRET_KEY` | Terkunci | Kunci tidak berada di DB |
| ADR-011 | Alert: evaluasi nilai terbaru + `for_seconds`, state machine pending/firing/resolved | Terkunci | Sederhana dan dapat diprediksi |
| ADR-012 | Notifikasi via antrean di DB + worker + retry backoff | Terkunci | Tahan restart, tidak menghambat ingest |
| ADR-013 | Retensi raw 14 hari, hourly 365 hari | Default | Ukuran DB ±2-4 GB + ±0,5 GB |
| ADR-014 | Role: `admin` dan `viewer` | Default | Cukup untuk tim kecil |
| ADR-015 | 2FA TOTP opsional, dikerjakan fase 10 | Default | Tidak menghalangi MVP |
| ADR-016 | Zona waktu tampilan default `Asia/Jakarta`, dapat diubah di Settings | Default | |
| ADR-017 | Bahasa UI dan notifikasi: Indonesia; kode: Inggris | Terkunci | |
| ADR-018 | Pengelompokan offline: ≥ 5 server dalam 120 dtk → satu ringkasan | Default | Mencegah banjir saat insiden jaringan |
| ADR-019 | Backup: Litestream ke S3-compatible + `VACUUM INTO` harian | Default | RPO ±1 menit |
| ADR-020 | Modul Go: `github.com/nocturnalismee/luma-monitoring/backend` | Default | Mengikuti nama repo pada script agent |
| ADR-021 | Satuan jaringan UI = bit/detik; I/O disk = byte/detik | Terkunci | Konvensi umum |
| ADR-022 | Tabel metrik memakai PK `(server_id, ts, subject)` dan `WITHOUT ROWID` | Terkunci | Retensi per server memakai indeks |

## 2. Pertanyaan Terbuka (untuk pengguna)

AI yang menemui ketidakjelasan menambahkan pertanyaan di tabel ini (format: `Q-nnn`) lalu bertanya kepada pengguna. Jangan memutuskan sendiri.

| ID | Pertanyaan | Default sementara | Dibutuhkan oleh |
|---|---|---|---|
| Q-001 | **Lisensi script agent.** Apakah lisensi asal (turunan agent HetrixTools) mengizinkan modifikasi, rebranding, dan distribusi? Apakah atribusi harus dipertahankan? | Pertahankan seluruh header hak cipta/disclaimer yang ada | T-001 |
| Q-002 | Penyedia penyimpanan backup (S3 AWS, Backblaze B2, MinIO)? | Backblaze B2 (S3-compatible) | T-093 |
| Q-003 | Penyedia SMTP yang akan dipakai? | Dikonfigurasi dari UI, tidak memblokir pengembangan | T-074 |
| Q-004 | Domain produksi final (`LUMA_BASE_URL`)? | `https://monitor.example.com` | T-004 |
| Q-005 | Apakah monitoring website/HTTP diperlukan di fase berikutnya? | Tidak (di luar cakupan v1) | - |
| Q-006 | Fitur agent opsional yang akan diaktifkan di server produksi (RAID, drive health, outgoing ping, daftar proses)? | Semua mati kecuali `CheckReboot` | T-042 |
| Q-007 | Apakah daftar proses (`RunningProcesses`) perlu ditampilkan di dashboard? | Tidak (tidak disimpan) | - |
| Q-008 | **Prasyarat T-001 belum terpenuhi.** `TASKS.md` mensyaratkan T-000 (bootstrap repo: `Makefile`, `backend/go.mod`, CI) sebelum T-001, tetapi T-000 masih `[ ]` dan tidak ada `Makefile`/`go.mod` di repo. T-001 dikerjakan lebih dulu sehingga kriteria "make lint / make test hijau" tidak dapat diverifikasi; verifikasi dilakukan dengan `grep`, `bash -n`, dan `git diff`. Apakah urutan itu disetujui, atau T-000 harus diselesaikan lebih dulu sebelum commit T-001 diterima? | T-001 dianggap selesai dengan verifikasi manual; T-000 menjadi tugas berikutnya yang wajib hijau sebelum T-002 | T-000, T-001 |
| Q-009 | **Ketepatan naskah `DEPLOYMENT.md` 4.2.** Dua hal di blok perintah bagian itu: (a) pola `sed` ditulis dengan `$BRANCH` tanpa escape sehingga bergantung pada perluasan variabel shell yang kosong — salinan yang lebih eksplisit adalah `\$BRANCH`; (b) pemeriksaan penerimaan memakai `grep -rn 'sm.luma.local\|raw.githubusercontent' agent/`, tetapi kedua literal itu masih tertulis di dalam teks `DEPLOYMENT.md` dan `DECISIONS.md` sendiri, jadi greppinya harus dibatasi ke file yang diperiksa (`agent/*.sh`). Apakah naskah `DEPLOYMENT.md` 4.2 boleh dikoreksi (usulkan: `docs(deployment): clarify T-001 sed and grep commands`) ? | Perintah dijalankan dengan pola ter-escape dan `grep` dibatasi ke `agent/*.sh`; koreksi naskah menunggu persetujuan pengguna | T-001, T-051 |

## 3. Catatan Audit Agent (informasi, bukan keputusan)

- `luma_update.sh` tidak menyalin nilai `DEBUG` dari cfg lama (kembali ke default `0` setelah update).
- Beberapa jalur error di installer/updater memakai `exit` tanpa kode → otomasi (Ansible, loop SSH) tidak dapat mendeteksi kegagalan lewat exit code; verifikasi lewat dashboard.
- Agent membuang respons HTTP; kegagalan kirim hanya terlihat sebagai server `offline`.
- `cpucores` pada payload sebenarnya jumlah **CPU logis**.
- Suhu yang tidak terbaca dikirim sebagai `0`.
- Perbedaan satuan RTT antara ICMP (µs) dan TCP (ms) pada field `oping`.
