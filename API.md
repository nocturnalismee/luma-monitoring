# API.md — Kontrak REST

> Basis: `/api`. Format JSON UTF-8. Semua `ts`/`*_at` = integer detik Unix UTC. Nama field `snake_case`.
> Frontend dan backend **harus** mengikuti dokumen ini; perubahan kontrak = ubah dokumen ini lebih dulu.

## 1. Konvensi Umum

**Autentikasi:** cookie sesi `luma_session` (diset oleh `/api/auth/login`). Metode non-GET wajib header `Content-Type: application/json` dan `Origin` yang cocok dengan host. Tanpa sesi → `401`.

**Peran:** `viewer` (baca dashboard) dan `admin` (semua). Kolom "Peran" di bawah = peran **minimum**.

**Format error (semua status ≥ 400):**

```json
{ "error": { "code": "validation_failed", "message": "Pesan manusiawi (Indonesia)", "fields": { "name": "wajib diisi" } } }
```

| HTTP | `code` | Arti |
|---|---|---|
| 400 | `bad_request` / `validation_failed` | Input salah (`fields` opsional) |
| 401 | `unauthenticated` | Belum login / sesi habis |
| 403 | `forbidden` | Peran tidak cukup / Origin tidak cocok |
| 404 | `not_found` | |
| 409 | `conflict` | Duplikat (mis. nama server) |
| 413 | `too_large` | |
| 429 | `rate_limited` | Sertakan header `Retry-After` |
| 500 | `internal` | Tanpa detail internal |
| 503 | `unavailable` | |

**Pagination:** `?limit=` (default 50, maksimal 200) dan `?offset=`; respons daftar: `{ "items": [...], "total": 123 }`.

**Waktu:** `from`/`to` = epoch detik. Pintasan `range` = `1h | 6h | 24h | 7d | 30d` (relatif ke sekarang). Jika keduanya diberikan, `from`/`to` menang.

## 2. Endpoint Publik (tanpa sesi)

| Metode + Path | Fungsi | Respons |
|---|---|---|
| `GET /healthz` | Kesehatan | `200 {"status":"ok","version":"x.y.z"}` atau `503 {"status":"degraded","reason":"db"}` |
| `POST /v2/{$}` | Ingest agent (`PROTOCOL.md`) | `200 ok` teks; `400/403/413/429/503` |
| `POST /{$}` | Event install/uninstall | `200 ok`; `400/403` |
| `GET\|HEAD /agent/{file}` | Script agent (`DEPLOYMENT.md` bagian 4) | file teks; `404` bila bukan allowlist |
| `GET /` dan path lain non-`/api` | SPA (fallback `index.html`) | HTML/asset |

## 3. Autentikasi

| Metode + Path | Peran | Body → Respons |
|---|---|---|
| `POST /api/auth/login` | - | `{"email","password","totp"?}` → `200 {"user":User}` + cookie. Gagal: `401` pesan generik |
| `POST /api/auth/logout` | viewer | → `204` |
| `GET /api/me` | viewer | → `200 {"user":User}` |

`User` = `{"id":1,"email":"a@b.c","role":"admin","totp_enabled":false}`.

## 4. Dashboard

### 4.1 `GET /api/overview` (viewer)

```json
{
  "generated_at": 1763100000,
  "servers": { "total": 60, "online": 57, "offline": 2, "pending": 1, "decommissioned": 0, "maintenance": 1 },
  "alerts":  { "firing": 3, "critical": 1, "warning": 2, "info": 0 },
  "avg_cpu": 23.4, "avg_ram": 51.2,
  "agent_latest_version": "2.4.2"
}
```

`maintenance` adalah hitungan terpisah (overlay), tidak mengurangi `online/offline`. `agent_latest_version` dibaca dari baris `Version=` agent yang di-*embed* (`DEPLOYMENT.md` bagian 4.4); UI memakainya untuk menandai agent usang.

### 4.2 `GET /api/servers` (viewer)

Query: `status`, `group_id`, `q` (cari nama/hostname), `limit`, `offset`, `sort` (`name|status|cpu|ram|last_seen`, awalan `-` = menurun).

```json
{
  "total": 60,
  "items": [{
    "id": 12, "name": "web-01", "group": { "id": 2, "name": "Web" },
    "status": "online", "maintenance": false, "last_seen": 1763099990,
    "os": "Ubuntu 22.04.4 LTS", "uptime_s": 1234567, "agent_version": "2.4.2",
    "cpu": 12.5, "ram": 41.2, "swap": 0, "load1": 0.35, "disk_max_pct": 41.0,
    "alerts_firing": 0,
    "spark": { "cpu": [10.1, 12.0, 11.3], "ram": [40.0, 41.1, 41.2] }
  }]
}
```

`spark`: 12 titik (60 menit, bucket 5 menit); `null` bila tidak ada data. `cpu/ram/...` = sample terakhir (boleh `null` untuk `pending`).

### 4.3 `GET /api/servers/{id}` (viewer)

```json
{
  "id": 12, "name": "web-01", "group": { "id": 2, "name": "Web" },
  "status": "online", "maintenance_until": null, "run_as_root": false,
  "hostname": "web-01", "os": "...", "kernel": "...", "cpu_model": "...",
  "cpu_sockets": 1, "cpu_logical": 4, "cpu_threads_per_core": 2, "cpu_mhz": 2400,
  "ram_size_kb": 8123456, "swap_size_kb": 2097148,
  "agent_version": "2.4.2", "agent_user": "luma", "req_reboot": false,
  "first_seen": 1760000000, "last_seen": 1763099990, "uptime_s": 1234567,
  "ips": [{ "iface": "eth0", "family": 4, "address": "203.0.113.10" }]
}
```

### 4.4 Deret waktu per server (viewer)

Parameter umum: `range` atau `from`/`to`; `step` opsional (detik, kelipatan 60). Jika `step` tidak diberikan, backend memilih `step = max(60, ceil((to-from)/500 / 60) × 60)` agar **≤ ~500 titik per seri**.

**Sumber data otomatis:** `to-from ≤ 14 hari` → tabel raw; `> 14 hari` → tabel `*_hourly` (step minimal 3600). Respons menyertakan `"source": "raw" | "hourly"`.

| Path | Bentuk |
|---|---|
| `GET /api/servers/{id}/metrics` | **Titik datar** |
| `GET /api/servers/{id}/network` | **Seri berkunci**, `key`=iface |
| `GET /api/servers/{id}/disk-io` | **Seri berkunci**, `key`=mount (hanya raw) |
| `GET /api/servers/{id}/temps` | **Seri berkunci**, `key`=sensor (hanya raw) |
| `GET /api/servers/{id}/ping` | **Seri berkunci**, `key`=target_name (hanya raw) |
| `GET /api/servers/{id}/connections` | **Seri berkunci**, `key`=port (hanya raw) |

Endpoint "hanya raw" untuk rentang > 14 hari mengembalikan data kosong dengan `"source":"none"`.

**Titik datar (`metrics`):**

```json
{ "from": 1763013600, "to": 1763100000, "step": 180, "source": "raw",
  "points": [{ "ts": 1763013600, "cpu": 12.5, "cpu_us": 9.1, "cpu_sy": 3.0, "cpu_wa": 0.4, "cpu_st": 0,
               "load1": 0.35, "load5": 0.4, "load15": 0.38,
               "ram": 41.2, "ram_buff": 1.1, "ram_cache": 18.7, "swap": 0 }] }
```

Untuk `source:"hourly"`: hanya `cpu`, `ram`, `swap`, `load1` (rata-rata) plus `cpu_max`, `ram_max`, `load1_max`; field lain `null`.

**Seri berkunci:**

```json
{ "from": 0, "to": 0, "step": 180, "source": "raw",
  "series": [{ "key": "eth0", "points": [{ "ts": 1763013600, "rx": 12000, "tx": 34000 }] }] }
```

Isi `points` per endpoint:

| Endpoint | Field titik | Satuan |
|---|---|---|
| network | `rx`, `tx` | bytes/detik |
| disk-io | `read`, `write` | bytes/detik |
| temps | `celsius` | °C |
| ping | `loss`, `rtt_ms` | %, ms (dari `rtt_us / 1000`) |
| connections | `count` | koneksi |

Agregasi downsampling: `AVG` (kecuali `*_max`).

### 4.5 `GET /api/servers/{id}/state` (viewer)

Mengembalikan `snapshot_json` (`DATABASE.md` bagian 4) ditambah `"updated_at"`. Untuk admin, tambahan `"hardware_raw"`: `{ "drives":[{"device","raw"}], "raid":[{"device","raw"}], "zpools":[{"pool","raw"}] }` (teks mentah; viewer tidak menerima field ini).

### 4.6 `GET /api/servers/{id}/events` (viewer)

→ `{ "items":[{ "ts":..., "type":"offline", "detail":"..." }], "total":N }` (terbaru dulu).

### 4.7 `GET /api/groups` (viewer) → `{ "items":[{ "id":2, "name":"Web", "server_count":5 }] }`

## 5. Manajemen Server (admin)

| Metode + Path | Body → Respons |
|---|---|
| `POST /api/servers` | `{"name","group_id"?,"run_as_root"?}` → `201 {"server":Server,"sid":"<32 hex>","commands":Commands}` — **SID hanya muncul di respons ini** |
| `PUT /api/servers/{id}` | `{"name"?,"group_id"?,"run_as_root"?}` → `200 {"server":Server}` |
| `DELETE /api/servers/{id}` | → `204` (menghapus semua datanya) |
| `POST /api/servers/{id}/rotate-sid` | → `200 {"sid":"...","commands":Commands}`; SID lama langsung tidak valid; status → `pending` |
| `GET /api/servers/{id}/commands` | → `200 {"commands":Commands}` **tanpa SID** (placeholder `<SID>`) |
| `POST /api/servers/{id}/maintenance` | `{"until": 1763200000 \| null}` → `200`; `null` = akhiri maintenance |
| `POST /api/groups` / `PUT /api/groups/{id}` / `DELETE /api/groups/{id}` | `{"name"}` → grup |

`Commands` (string siap salin; `BASE` = `LUMA_BASE_URL`):

```json
{
  "install":   "wget -qO luma_install.sh BASE/agent/luma_install.sh && bash luma_install.sh <SID> <1 jika run_as_root, 0 jika tidak>",
  "update":    "wget -qO luma_update.sh BASE/agent/luma_update.sh && bash luma_update.sh",
  "uninstall": "wget -qO luma_uninstall.sh BASE/agent/luma_uninstall.sh && bash luma_uninstall.sh <SID>"
}
```

Validasi: `name` 1-64 karakter, unik. SID dibuat dari 16 byte acak (`crypto/rand`) → 32 karakter hex.

## 6. Alert

### 6.1 Event alert

`GET /api/alerts` (viewer) — query: `state` (`pending|firing|resolved`, boleh koma), `severity`, `server_id`, `from`, `to`, `limit`, `offset`.

```json
{ "total": 3, "items": [{
  "id": 77, "rule": { "id": 4, "name": "Disk hampir penuh" },
  "server": { "id": 12, "name": "web-01" }, "subject": "/",
  "severity": "warning", "state": "firing",
  "started_at": 1763090000, "fired_at": 1763090300, "resolved_at": null,
  "last_value": 87.2, "message": "Disk / 87.2% > 85%", "acked": false
}]}
```

`POST /api/alerts/{id}/ack` (admin) → `200` (set `acked_at`, `acked_by`).

### 6.2 Aturan

| Metode + Path | Peran | Keterangan |
|---|---|---|
| `GET /api/alert-rules` | viewer | `{"items":[Rule]}` |
| `POST /api/alert-rules` | admin | body = Rule (tanpa `id`) → `201 Rule` |
| `PUT /api/alert-rules/{id}` | admin | → `200 Rule` |
| `DELETE /api/alert-rules/{id}` | admin | → `204` |

`Rule`:

```json
{ "id": 4, "name": "Disk hampir penuh", "kind": "threshold", "metric": "disk_pct", "op": ">", "threshold": 85,
  "recover_threshold": null, "for_seconds": 300, "severity": "warning",
  "scope_type": "all", "scope_id": null, "subject_exclude": "^/(boot|snap|run)",
  "repeat_seconds": 3600, "notify_resolved": true, "enabled": true, "channel_ids": [1, 2] }
```

Validasi: kombinasi `kind`/`metric`/`op`/`threshold` mengikuti `CHECK` di `DATABASE.md`; `subject_exclude` harus regex Go valid; `for_seconds ≥ 0`; `repeat_seconds ≥ 300`.

## 7. Kanal Notifikasi (admin)

| Metode + Path | Keterangan |
|---|---|
| `GET /api/channels` | `{"items":[Channel]}` (secret bertopeng) |
| `POST /api/channels` | buat → `201 Channel` |
| `PUT /api/channels/{id}` | ubah. Field secret bernilai `"********"` atau tidak dikirim = **pertahankan nilai lama** |
| `DELETE /api/channels/{id}` | `204` |
| `POST /api/channels/{id}/test` | kirim pesan tes → `{"ok":true}` atau `{"ok":false,"error":"<pesan tersanitasi>"}` (selalu `200`) |
| `POST /api/channels/telegram/detect-chats` | body `{"bot_token"?|"channel_id"?}` → `{"chats":[{"id":-100123,"title":"Ops","type":"group"}]}` |

`Channel` (Telegram):

```json
{ "id": 1, "name": "Ops Telegram", "type": "telegram", "enabled": true, "min_severity": "warning",
  "config": { "bot_token": "********", "chat_id": "-100123456", "silent": false },
  "last_error": null, "last_ok_at": 1763090000 }
```

`Channel` (Email) `config`:

```json
{ "host": "smtp.example.com", "port": 587, "security": "starttls", "username": "user",
  "password": "********", "from": "monitor@example.com", "to": ["ops@example.com"] }
```

`security` ∈ `starttls | tls | none`. Validasi: `chat_id` wajib; `to` minimal satu alamat valid; `port` 1-65535. API **tidak pernah** mengembalikan secret asli, dan tidak mencatatnya ke log atau audit.

## 8. Log Notifikasi dan Pengaturan

| Metode + Path | Peran | Keterangan |
|---|---|---|
| `GET /api/notifications/log` | admin | query `status`, `channel_id`, `limit`, `offset` → `{"items":[{"id","ts","channel":{"id","name"},"kind","status","attempts","last_error","title"}],"total"}` |
| `POST /api/notifications/log/{id}/retry` | admin | hanya `dead` → status `queued`, `attempts=0` |
| `GET /api/settings` | viewer | `{"display_tz":"Asia/Jakarta","ui_thresholds":{...}}` (nilai non-sensitif) |
| `GET /api/settings/admin` | admin | semua kunci `app_settings` (`DATABASE.md` bagian 6) |
| `PUT /api/settings/admin` | admin | objek parsial kunci yang valid; kunci asing → `400` |

## 9. Pengguna dan Audit (admin)

| Metode + Path | Keterangan |
|---|---|
| `GET /api/users` | `{"items":[User]}` |
| `POST /api/users` | `{"email","password","role"}` (password ≥ 12) → `201 User` |
| `PUT /api/users/{id}` | `{"role"?,"password"?}` ; tidak boleh menurunkan admin terakhir |
| `DELETE /api/users/{id}` | `204`; tidak boleh menghapus diri sendiri atau admin terakhir |
| `GET /api/audit-log` | `limit/offset/from/to/action` → `{"items":[{"ts","user","action","target","detail","ip"}],"total"}` |

## 10. Rate Limit (default)

| Rute | Batas |
|---|---|
| `POST /v2/`, `POST /` | 10/menit per SID, 600/menit per IP |
| `POST /api/auth/login` | 5/menit per IP (+ penguncian akun) |
| `POST /api/channels/*/test`, `detect-chats` | 10/menit per pengguna |
| `GET /api/*` | 300/menit per pengguna |
