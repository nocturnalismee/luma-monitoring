# DEPLOYMENT.md — Build, Deploy (Coolify), Distribusi Agent, Backup

## 1. Environment Variable

Semua dibaca saat start oleh `internal/config`. Variabel wajib yang hilang/invalid → proses **gagal start** dengan pesan jelas (tanpa mencetak nilai secret).

| Variabel | Wajib | Default | Keterangan |
|---|---|---|---|
| `LUMA_BASE_URL` | ya | - | URL publik tanpa slash akhir, mis. `https://monitor.example.com`. Dipakai untuk templating agent dan tautan notifikasi. Produksi wajib `https://` |
| `LUMA_SECRET_KEY` | ya | - | 32 byte acak dalam base64 (`openssl rand -base64 32`). Kunci AES-GCM untuk secret kanal dan TOTP. **Simpan di password manager.** Tidak ada rotasi otomatis |
| `LUMA_DB_PATH` | tidak | `/data/luma.db` | Lokasi SQLite |
| `LUMA_LISTEN` | tidak | `:8080` | Alamat listen |
| `LUMA_TRUSTED_PROXIES` | tidak | `127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16` | CIDR proxy yang dipercaya untuk `X-Forwarded-For` |
| `LUMA_ADMIN_EMAIL`, `LUMA_ADMIN_PASSWORD` | hanya run pertama | - | Membuat admin pertama **jika tabel `users` kosong**; lalu abaikan. Password ≥ 12 karakter |
| `LUMA_INGEST_MAX_BODY` | tidak | `5242880` | Batas body ingest (byte) |
| `LUMA_INGEST_MAX_DECOMPRESSED` | tidak | `20971520` | Batas hasil gunzip (byte) |
| `LUMA_OFFLINE_AFTER` | tidak | `180` | Detik tanpa data sebelum server `offline` |
| `LUMA_SESSION_TTL` | tidak | `12h` | Format `time.ParseDuration` |
| `LUMA_RETENTION_RAW_DAYS` | tidak | `14` | |
| `LUMA_RETENTION_HOURLY_DAYS` | tidak | `365` | |
| `LUMA_LOG_LEVEL` | tidak | `info` | `debug|info|warn|error` |
| `LITESTREAM_*` | opsional | - | Lihat bagian 6 |

Sediakan `.env.example` di repo berisi variabel di atas **tanpa nilai rahasia**.

## 2. Dockerfile (multi-stage)

Versi Go/Node/Alpine adalah contoh; samakan dengan `go.mod` dan `.nvmrc` proyek.

```dockerfile
# --- Tahap 1: frontend ---
FROM node:20-alpine AS web
WORKDIR /web
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# --- Tahap 2: backend (embed frontend dan agent) ---
FROM golang:1.23-alpine AS api
WORKDIR /src
COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download
COPY backend/ ./backend/
COPY agent/ ./agent/
RUN mkdir -p backend/internal/agentdist/files && cp agent/* backend/internal/agentdist/files/
COPY --from=web /web/dist ./backend/internal/web/dist
RUN cd backend && CGO_ENABLED=0 go build -ldflags="-s -w" -o /luma ./cmd/luma

# --- Tahap 3: image akhir ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 10001 app && mkdir -p /data && chown app:app /data
COPY --from=api /luma /luma
USER app
VOLUME /data
ENV LUMA_DB_PATH=/data/luma.db
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=20s \
  CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/luma"]
```

`ca-certificates` **wajib** (backend memanggil Telegram dan SMTP lewat TLS). Sediakan `.dockerignore` (`.git`, `node_modules`, `*.db`, `docs`).

## 3. Deploy di Coolify

1. Push repo ke Git (GitHub/GitLab/Gitea).
2. **New Resource → Application**, pilih repo, *Build Pack* = **Dockerfile**, base directory `/`.
3. **Ports Exposes** = `8080`.
4. **Domains** = `https://monitor.example.com` (arahkan DNS A ke IP VPS lebih dulu).
5. **Persistent Storage**: volume ke `/data` (**wajib**). Gunakan *named volume* agar kepemilikan `app:app` dari image terbawa; bila memakai bind mount dan muncul `permission denied`, jalankan `chown 10001:10001` pada direktori host.
6. **Environment Variables**: isi tabel bagian 1; tandai `LUMA_SECRET_KEY`, `LUMA_ADMIN_PASSWORD`, `LITESTREAM_*` sebagai secret dan **runtime only** (bukan build-time).
7. **Health Check**: path `/healthz`, port `8080`.
8. Aktifkan **auto-deploy** (webhook) bila diinginkan.
9. **Jangan** menaikkan jumlah replika (SQLite satu penulis).

Verifikasi setelah deploy: lihat bagian 7.

## 4. Distribusi Agent dari Backend

### 4.1 Prinsip

Script agent disimpan di repo (`agent/`), di-*embed* ke binary, dan disajikan di `/agent/{file}` setelah string `__LUMA_BASE_URL__` diganti dengan `LUMA_BASE_URL`. Hasil templating dihitung sekali saat start dan di-cache di memori.

Karena `go:embed` tidak boleh merujuk di luar modul, `agent/*` disalin ke `backend/internal/agentdist/files/` oleh `make sync-agent` (dan oleh Dockerfile). Direktori `files/` di-*gitignore* kecuali placeholder `files/README.txt` agar `embed` tidak kosong. `make test` dan `make build` bergantung pada `sync-agent`.

### 4.2 Perubahan pada script (satu kali, tugas T-001)

**Hanya** penggantian URL berikut. Jangan mengubah logika lain.

| File | Dari | Ke |
|---|---|---|
| `luma_agent.sh` (2 tempat) | `https://sm.luma.local/v2/` | `__LUMA_BASE_URL__/v2/` |
| `luma_install.sh` | `https://sm.luma.local/` | `__LUMA_BASE_URL__/` |
| `luma_install.sh` (3 tempat: spider, agent, cfg) | `https://raw.githubusercontent.com/nocturnalismee/luma-monitoring/$BRANCH/agent` | `__LUMA_BASE_URL__/agent` |
| `luma_update.sh` (2 tempat: agent, cfg) | `https://raw.githubusercontent.com/nocturnalismee/luma-monitoring/$BRANCH/agent` | `__LUMA_BASE_URL__/agent` |
| `luma_uninstall.sh` | `https://sm.luma.local/` | `__LUMA_BASE_URL__/` |

Perintah (jalankan dari root repo):

```bash
sed -i 's#https://sm.luma.local#__LUMA_BASE_URL__#g' agent/luma_agent.sh agent/luma_install.sh agent/luma_uninstall.sh
sed -i 's#https://raw.githubusercontent.com/nocturnalismee/luma-monitoring/\$BRANCH/agent#__LUMA_BASE_URL__/agent#g' agent/luma_install.sh agent/luma_update.sh
grep -rn 'sm.luma.local\|raw.githubusercontent' agent/ && echo "MASIH ADA YANG BELUM DIGANTI" || echo "OK"
for f in agent/*.sh; do bash -n "$f" || echo "SYNTAX ERROR $f"; done
```

Catatan perilaku yang **tidak** diubah: argumen cabang (`BRANCH`) pada installer tetap diparsing tetapi tidak lagi memengaruhi URL; `rm -rf /etc/luma` saat instal ulang tetap menghapus `custom_variables.json` (gunakan `luma_update.sh` untuk pembaruan).

### 4.3 Endpoint

| Path | Isi |
|---|---|
| `/agent/luma_agent.sh`, `/agent/luma.cfg`, `/agent/luma_install.sh`, `/agent/luma_update.sh`, `/agent/luma_uninstall.sh` | File hasil templating |
| `/agent/SHA256SUMS` | Format `sha256sum` untuk kelima file di atas (hash dihitung **setelah** templating) |

Aturan handler: allowlist nama file persis (selain itu `404`); `Content-Type: text/plain; charset=utf-8`; `Cache-Control: no-cache`; dukung `HEAD` (installer memakai `wget --spider`); tanpa autentikasi; path traversal mustahil karena hanya lookup map.

### 4.4 Versi agent

Backend membaca baris `Version="x.y.z"` dari `luma_agent.sh` saat start (regex sama dengan `extract_agent_version` di `luma_update.sh`) dan mengeksposnya sebagai `agent_latest_version` di `GET /api/overview`. UI menandai server dengan `agent_version` berbeda sebagai "usang". Pembaruan agent dilakukan manual lewat perintah `update` (tidak ada auto-update).

### 4.5 Perintah untuk pengguna

Dihasilkan oleh `API.md` bagian 5. Contoh:

```bash
wget -qO luma_install.sh https://monitor.example.com/agent/luma_install.sh && bash luma_install.sh <SID32> 0
```

Argumen ke-2: `1` = root, `0` = user `luma`. SID tampil di baris perintah dan riwayat shell; hapus riwayat bila perlu, atau gunakan *rotate SID* bila bocor.

## 5. Rollout ke 60 Server

1. **Canary**: 3-5 server beragam (Debian/Ubuntu, RHEL, dengan RAID/NVMe bila ada). Amati 24-72 jam: data masuk, ukuran DB, perilaku alert.
2. Tambah server di UI, salin perintah instalasi, jalankan sebagai root di server tujuan.
3. Gelombang 10 server per hari; pantau `ingest_rejections` dan `server_events`.
4. Agent lama (jika ada) boleh berdampingan selama transisi (direktori berbeda); hapus setelah yakin.
5. Setelah selesai: aktifkan aturan alert dan kanal secara bertahap (mulai dengan `critical` saja untuk mencegah banjir awal).

## 6. Backup dan Pemulihan (Litestream)

Variabel: `LITESTREAM_BUCKET`, `LITESTREAM_ENDPOINT` (kosongkan untuk AWS S3), `LITESTREAM_ACCESS_KEY_ID`, `LITESTREAM_SECRET_ACCESS_KEY`.

`litestream.yml`:

```yaml
dbs:
  - path: /data/luma.db
    replicas:
      - type: s3
        bucket: ${LITESTREAM_BUCKET}
        path: luma
        endpoint: ${LITESTREAM_ENDPOINT}
        access-key-id: ${LITESTREAM_ACCESS_KEY_ID}
        secret-access-key: ${LITESTREAM_SECRET_ACCESS_KEY}
```

`scripts/entrypoint.sh` (dipakai bila `LITESTREAM_BUCKET` terisi):

```sh
#!/bin/sh
set -e
if [ -n "$LITESTREAM_BUCKET" ]; then
  [ -f "$LUMA_DB_PATH" ] || litestream restore -if-replica-exists -config /etc/litestream.yml "$LUMA_DB_PATH"
  exec litestream replicate -config /etc/litestream.yml -exec /luma
fi
exec /luma
```

Pasang binary Litestream di image akhir dengan **versi rilis stabil yang dipin** (periksa halaman rilis resmi saat mengerjakan T-093) dan ubah `ENTRYPOINT` ke skrip di atas.

Pemulihan (**wajib diuji sebelum produksi**): hentikan aplikasi → hapus `/data/luma.db*` → `litestream restore` → pastikan `LUMA_SECRET_KEY` sama → start → verifikasi data terbaru. Target RPO ±1 menit, RTO < 30 menit.

Cadangan tambahan: job harian `VACUUM INTO '/data/backup/luma-YYYYMMDD.db'` (simpan 7 hari) dan salin keluar VPS.

## 7. Smoke Test Pasca-Deploy

```bash
# 1. Kesehatan
curl -fsS https://monitor.example.com/healthz

# 2. Agent file tersedia dan ter-templating (tidak boleh ada __LUMA_BASE_URL__ tersisa)
curl -fsS https://monitor.example.com/agent/luma_agent.sh | grep -c '__LUMA_BASE_URL__'   # harus 0
curl -fsS https://monitor.example.com/agent/SHA256SUMS

# 3. SID tidak dikenal ditolak
curl -s -o /dev/null -w '%{http_code}\n' -X POST -d 'v=install&s=00000000000000000000000000000000' https://monitor.example.com/   # 403

# 4. Replay fixture dengan SID server uji
curl --data-binary @backend/testdata/payloads/ubuntu.body https://monitor.example.com/v2/   # 200 ok
```

Lalu: login ke UI, pastikan server uji `online` dan grafik terisi.

## 8. Operasional

| Tugas | Cara |
|---|---|
| Upgrade aplikasi | Push ke Git → Coolify redeploy. Migrasi berjalan otomatis; backup `pre-NNNN.db` dibuat sebelum migrasi |
| Rollback | Deploy tag lama + restore backup `pre-NNNN.db` (migrasi bersifat maju saja) |
| Lihat log | Log container di Coolify (JSON) |
| Pantau disk | Pasang agent di VPS monitor sendiri; aturan disk bawaan berlaku |
| Pemantauan eksternal | Uptime Kuma (di host lain) ke `/healthz` |
| SID bocor | UI → Rotate SID, jalankan ulang perintah instalasi pada server tersebut |
| Kehilangan `LUMA_SECRET_KEY` | Kanal notifikasi dan TOTP tidak terbaca; buat ulang kanal, nonaktifkan TOTP lewat DB oleh admin |
