# DATABASE.md — Skema SQLite

> Mesin: SQLite (WAL) via `modernc.org/sqlite`. Satu penulis, banyak pembaca.
> Semua `ts` = integer detik Unix UTC. Boolean = `INTEGER` 0/1. Enum = `TEXT` + `CHECK`.

## 1. Koneksi dan Pragma

DSN (modernc):

```
file:/data/luma.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)
```

| Handle | Pengaturan |
|---|---|
| **Writer** | `sql.DB` terpisah, `SetMaxOpenConns(1)`; hanya dipakai goroutine writer dan job penulis (satu pintu: paket `store` melalui `store.Writer`) |
| **Reader** | `sql.DB` terpisah dengan DSN `mode=ro`, `SetMaxOpenConns(4)` |

Job lain yang perlu menulis (evaluator, retensi, API admin) **mengirim fungsi ke antrean writer** (`writer.Do(func(tx *sql.Tx) error)`), tidak membuka koneksi tulis sendiri.

## 2. Aturan Migrasi

- File di `backend/migrations/NNNN_nama.sql` (4 digit), di-*embed*, dijalankan berurutan saat start dalam satu transaksi per file.
- Tabel `schema_migrations(version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL)` dibuat oleh runner, **bukan** oleh file migrasi.
- **Dilarang mengubah migrasi yang sudah ada.** Perubahan = file baru.
- Migrasi hanya "maju" (tanpa *down*). Cadangan otomatis sebelum migrasi mayor: `VACUUM INTO` ke `/data/backup/pre-NNNN.db`.
- Seed (aturan alert bawaan) dilakukan di migrasi `0002_seed_rules.sql`, bukan di kode.

## 3. Skema (`0001_init.sql`)

Urutan tabel sudah memperhatikan referensi FK.

```sql
-- ===== Inventaris =====
CREATE TABLE server_groups (
  id   INTEGER PRIMARY KEY,
  name TEXT NOT NULL UNIQUE
);

CREATE TABLE servers (
  id                INTEGER PRIMARY KEY,
  name              TEXT NOT NULL UNIQUE,
  sid_hash          TEXT NOT NULL UNIQUE,            -- SHA-256 hex dari SID
  group_id          INTEGER REFERENCES server_groups(id) ON DELETE SET NULL,
  run_as_root       INTEGER NOT NULL DEFAULT 0,      -- menentukan argumen ke-2 perintah install
  status            TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','online','offline','decommissioned')),
  maintenance_until INTEGER,                         -- aktif jika > now
  hostname          TEXT, os TEXT, kernel TEXT,
  cpu_model         TEXT, cpu_sockets INTEGER,
  cpu_logical       INTEGER,                         -- dari field cpucores
  cpu_threads_per_core INTEGER, cpu_mhz INTEGER,
  ram_size_kb       INTEGER, swap_size_kb INTEGER,
  agent_version     TEXT, agent_user TEXT,
  req_reboot        INTEGER NOT NULL DEFAULT 0,
  first_seen        INTEGER,
  last_seen         INTEGER,
  created_at        INTEGER NOT NULL,
  updated_at        INTEGER NOT NULL
);
CREATE INDEX idx_servers_status ON servers(status);

CREATE TABLE server_ips (
  server_id INTEGER NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  iface     TEXT NOT NULL,
  family    INTEGER NOT NULL CHECK (family IN (4,6)),
  address   TEXT NOT NULL,
  PRIMARY KEY (server_id, iface, address)
);

CREATE TABLE server_events (
  id        INTEGER PRIMARY KEY,
  server_id INTEGER NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  ts        INTEGER NOT NULL,
  type      TEXT NOT NULL CHECK (type IN
            ('created','install','uninstall','first_data','online','offline',
             'sid_rotated','maintenance_start','maintenance_end','agent_version_changed')),
  detail    TEXT
);
CREATE INDEX idx_server_events ON server_events(server_id, ts);

-- ===== Metrik raw (per menit) =====
CREATE TABLE metrics (
  server_id INTEGER NOT NULL, ts INTEGER NOT NULL,
  cpu REAL, cpu_us REAL, cpu_sy REAL, cpu_wa REAL, cpu_st REAL,
  load1 REAL, load5 REAL, load15 REAL,
  ram REAL, ram_buff REAL, ram_cache REAL, swap REAL,
  uptime INTEGER,
  PRIMARY KEY (server_id, ts)
) WITHOUT ROWID;

CREATE TABLE net_usage (
  server_id INTEGER NOT NULL, ts INTEGER NOT NULL, iface TEXT NOT NULL,
  rx_bps INTEGER, tx_bps INTEGER,                     -- bytes/detik
  PRIMARY KEY (server_id, ts, iface)
) WITHOUT ROWID;

CREATE TABLE disk_io (
  server_id INTEGER NOT NULL, ts INTEGER NOT NULL, mount TEXT NOT NULL,
  read_bps INTEGER, write_bps INTEGER,                -- bytes/detik
  PRIMARY KEY (server_id, ts, mount)
) WITHOUT ROWID;

CREATE TABLE temps (
  server_id INTEGER NOT NULL, ts INTEGER NOT NULL, sensor TEXT NOT NULL,
  celsius REAL NOT NULL,
  PRIMARY KEY (server_id, ts, sensor)
) WITHOUT ROWID;

CREATE TABLE ping_results (
  server_id INTEGER NOT NULL, ts INTEGER NOT NULL, target_name TEXT NOT NULL,
  target TEXT NOT NULL, port INTEGER, proto TEXT NOT NULL CHECK (proto IN ('icmp','tcp')),
  loss_pct REAL, rtt_us INTEGER,                      -- sudah dinormalisasi ke mikrodetik
  PRIMARY KEY (server_id, ts, target_name)
) WITHOUT ROWID;

CREATE TABLE conn_counts (
  server_id INTEGER NOT NULL, ts INTEGER NOT NULL, port INTEGER NOT NULL,
  count INTEGER,
  PRIMARY KEY (server_id, ts, port)
) WITHOUT ROWID;

-- ===== Tabel "slow": ditulis maksimal tiap 5 menit per server =====
CREATE TABLE disk_usage (
  server_id INTEGER NOT NULL, ts INTEGER NOT NULL, mount TEXT NOT NULL,
  fstype TEXT, total INTEGER, used INTEGER, avail INTEGER,   -- bytes
  inode_total INTEGER, inode_used INTEGER,
  PRIMARY KEY (server_id, ts, mount)
) WITHOUT ROWID;

CREATE TABLE service_status (
  server_id INTEGER NOT NULL, ts INTEGER NOT NULL, name TEXT NOT NULL,
  running INTEGER NOT NULL,
  PRIMARY KEY (server_id, ts, name)
) WITHOUT ROWID;

-- ===== Keadaan terakhir (diperbarui tiap ingest) =====
CREATE TABLE server_state (
  server_id     INTEGER PRIMARY KEY REFERENCES servers(id) ON DELETE CASCADE,
  updated_at    INTEGER NOT NULL,
  snapshot_json TEXT NOT NULL            -- struktur: lihat bagian 4
);

-- ===== Hardware (nilai terakhir, upsert tiap 5 menit atau saat status berubah) =====
CREATE TABLE drive_health (
  server_id INTEGER NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  device TEXT NOT NULL, kind TEXT NOT NULL CHECK (kind IN ('smart','nvme','sas')),
  model TEXT, serial TEXT,
  status TEXT NOT NULL CHECK (status IN ('ok','fail','unknown')),
  raw TEXT, updated_at INTEGER NOT NULL,
  PRIMARY KEY (server_id, device)
);

CREATE TABLE raid_status (
  server_id INTEGER NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  mount TEXT NOT NULL, device TEXT NOT NULL,
  degraded INTEGER NOT NULL, raw TEXT, updated_at INTEGER NOT NULL,
  PRIMARY KEY (server_id, mount, device)
);

CREATE TABLE zpool_status (
  server_id INTEGER NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  pool TEXT NOT NULL, mount TEXT, health TEXT NOT NULL, raw TEXT, updated_at INTEGER NOT NULL,
  PRIMARY KEY (server_id, pool)
);

-- ===== Agregat jangka panjang (per jam) =====
CREATE TABLE metrics_hourly (
  server_id INTEGER NOT NULL, ts INTEGER NOT NULL,           -- awal jam (kelipatan 3600)
  cpu_avg REAL, cpu_max REAL, ram_avg REAL, ram_max REAL,
  swap_avg REAL, load1_avg REAL, load1_max REAL,
  PRIMARY KEY (server_id, ts)
) WITHOUT ROWID;

CREATE TABLE net_hourly (
  server_id INTEGER NOT NULL, ts INTEGER NOT NULL, iface TEXT NOT NULL,
  rx_avg INTEGER, rx_max INTEGER, tx_avg INTEGER, tx_max INTEGER,
  PRIMARY KEY (server_id, ts, iface)
) WITHOUT ROWID;

-- ===== Autentikasi dan audit =====
CREATE TABLE users (
  id INTEGER PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,                              -- argon2id (format PHC)
  role TEXT NOT NULL DEFAULT 'viewer' CHECK (role IN ('admin','viewer')),
  totp_secret_enc BLOB,                                     -- AES-GCM, NULL jika 2FA mati
  failed_logins INTEGER NOT NULL DEFAULT 0, locked_until INTEGER,
  created_at INTEGER NOT NULL, last_login_at INTEGER
);

CREATE TABLE sessions (
  id TEXT PRIMARY KEY,                                      -- SHA-256 hex dari token sesi
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at INTEGER NOT NULL, expires_at INTEGER NOT NULL,
  ip TEXT, user_agent TEXT
);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

CREATE TABLE audit_log (
  id INTEGER PRIMARY KEY, ts INTEGER NOT NULL, user_id INTEGER,
  action TEXT NOT NULL, target TEXT, detail TEXT, ip TEXT   -- detail TANPA secret
);
CREATE INDEX idx_audit_ts ON audit_log(ts);

CREATE TABLE ingest_rejections (
  id INTEGER PRIMARY KEY, ts INTEGER NOT NULL, ip TEXT,
  reason TEXT NOT NULL CHECK (reason IN
    ('unknown_sid','bad_payload','too_large','rate_limited','queue_full','bad_event'))
);
CREATE INDEX idx_rejections_ts ON ingest_rejections(ts);

-- ===== Notifikasi =====
CREATE TABLE notification_channels (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  type TEXT NOT NULL CHECK (type IN ('telegram','email')),
  config_enc BLOB NOT NULL,                                 -- nonce(12) || ciphertext, AES-256-GCM
  min_severity TEXT NOT NULL DEFAULT 'warning' CHECK (min_severity IN ('info','warning','critical')),
  enabled INTEGER NOT NULL DEFAULT 1,
  last_error TEXT, last_ok_at INTEGER,
  created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
);

-- ===== Alert =====
CREATE TABLE alert_rules (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN
    ('threshold','offline','service_down','reboot_required','raid_degraded','drive_unhealthy','zpool_unhealthy')),
  metric TEXT CHECK (metric IN
    ('cpu','ram','swap','load_per_core','disk_pct','inode_pct','temp','ping_loss','ping_rtt_ms')),
  op TEXT CHECK (op IN ('>','>=','<','<=')),
  threshold REAL,                       -- untuk 'offline': detik
  recover_threshold REAL,               -- hysteresis opsional (NULL = pakai threshold)
  for_seconds INTEGER NOT NULL DEFAULT 300,
  severity TEXT NOT NULL DEFAULT 'warning' CHECK (severity IN ('info','warning','critical')),
  scope_type TEXT NOT NULL DEFAULT 'all' CHECK (scope_type IN ('all','group','server')),
  scope_id INTEGER,                     -- group_id atau server_id
  subject_exclude TEXT,                 -- regex Go
  repeat_seconds INTEGER NOT NULL DEFAULT 3600,
  notify_resolved INTEGER NOT NULL DEFAULT 1,
  enabled INTEGER NOT NULL DEFAULT 1,
  created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,
  CHECK (kind <> 'threshold' OR (metric IS NOT NULL AND op IS NOT NULL AND threshold IS NOT NULL)),
  CHECK (scope_type = 'all' OR scope_id IS NOT NULL)
);

CREATE TABLE alert_rule_channels (
  rule_id INTEGER NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
  channel_id INTEGER NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
  PRIMARY KEY (rule_id, channel_id)
);

CREATE TABLE alert_events (
  id INTEGER PRIMARY KEY,
  rule_id INTEGER NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
  server_id INTEGER NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  subject TEXT NOT NULL DEFAULT '',     -- '' bila tidak ada subjek
  severity TEXT NOT NULL,
  state TEXT NOT NULL CHECK (state IN ('pending','firing','resolved')),
  started_at INTEGER NOT NULL, fired_at INTEGER, resolved_at INTEGER,
  last_notified_at INTEGER, last_value REAL, message TEXT,
  acked_by INTEGER REFERENCES users(id) ON DELETE SET NULL, acked_at INTEGER
);
-- Hanya satu event aktif per kunci deduplikasi:
CREATE UNIQUE INDEX uq_alert_active ON alert_events(rule_id, server_id, subject)
  WHERE state IN ('pending','firing');
CREATE INDEX idx_alert_state ON alert_events(state, server_id);
CREATE INDEX idx_alert_started ON alert_events(started_at);

CREATE TABLE notification_queue (
  id INTEGER PRIMARY KEY,
  channel_id INTEGER NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
  alert_event_id INTEGER REFERENCES alert_events(id) ON DELETE SET NULL,
  kind TEXT NOT NULL CHECK (kind IN ('firing','repeat','resolved','group','test')),
  payload TEXT NOT NULL,                -- JSON: {title, body_text, body_html, severity}
  status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','sent','dead')),
  attempts INTEGER NOT NULL DEFAULT 0,
  next_attempt_at INTEGER NOT NULL,
  last_error TEXT, created_at INTEGER NOT NULL, sent_at INTEGER
);
CREATE INDEX idx_nq_due ON notification_queue(status, next_attempt_at);

CREATE TABLE app_settings (
  key TEXT PRIMARY KEY, value TEXT NOT NULL                 -- value = JSON
);
```

## 4. Struktur `server_state.snapshot_json`

Dipakai UI (halaman detail) dan evaluator alert. Ditulis ulang **setiap ingest**.

```json
{
  "ts": 1763100000,
  "disks":    [{"mount":"/","fstype":"ext4","total":0,"used":0,"avail":0,"inode_total":0,"inode_used":0}],
  "services": [{"name":"nginx","running":true}],
  "temps":    [{"sensor":"x86_pkg_temp","celsius":45.0}],
  "ping":     [{"name":"Google","target":"8.8.8.8","port":null,"proto":"icmp","loss_pct":0,"rtt_us":12500}],
  "conn":     [{"port":443,"count":48}],
  "ips":      [{"iface":"eth0","family":4,"address":"203.0.113.10"}],
  "raid":     [{"mount":"/","device":"/dev/md0","degraded":false}],
  "zpools":   [{"pool":"tank","health":"ONLINE"}],
  "drives":   [{"device":"sda","kind":"smart","model":"...","serial":"...","status":"ok"}],
  "req_reboot": false
}
```

`raw` teks hardware **tidak** masuk snapshot (hanya di tabel `drive_health`, `raid_status`, `zpool_status`).

## 5. Pola Query Penting

Penghapusan retensi (per server, memakai PK; tabel `WITHOUT ROWID` tidak punya `rowid`, jadi batasi lewat subquery pada kolom PK):

```sql
DELETE FROM metrics WHERE server_id = ? AND ts < ? AND ts IN (
  SELECT ts FROM metrics WHERE server_id = ? AND ts < ? ORDER BY ts LIMIT 10000);
```

Untuk tabel dengan PK `(server_id, ts, subject)` (mis. `net_usage`), batasi dengan `ts` yang sama: ambil daftar `ts` berbeda terlama (`SELECT DISTINCT ts ... LIMIT 10000`) lalu hapus semua baris dengan `ts` tersebut. Ulangi per server dan per tabel sampai tidak ada baris terhapus.

Agregasi jam:

```sql
INSERT INTO metrics_hourly (server_id, ts, cpu_avg, cpu_max, ram_avg, ram_max, swap_avg, load1_avg, load1_max)
SELECT server_id, (ts/3600)*3600, AVG(cpu), MAX(cpu), AVG(ram), MAX(ram), AVG(swap), AVG(load1), MAX(load1)
FROM metrics WHERE ts >= ? AND ts < ?
GROUP BY server_id, ts/3600
ON CONFLICT(server_id, ts) DO UPDATE SET
  cpu_avg=excluded.cpu_avg, cpu_max=excluded.cpu_max, ram_avg=excluded.ram_avg,
  ram_max=excluded.ram_max, swap_avg=excluded.swap_avg,
  load1_avg=excluded.load1_avg, load1_max=excluded.load1_max;
```

Downsampling untuk grafik (bucket `step` detik):

```sql
SELECT (ts/?)*? AS t, AVG(cpu), AVG(cpu_us), AVG(cpu_sy), AVG(cpu_wa), AVG(cpu_st)
FROM metrics WHERE server_id = ? AND ts >= ? AND ts < ?
GROUP BY t ORDER BY t;
```

Upsert sample (idempotensi menit yang sama):

```sql
INSERT INTO metrics (...) VALUES (...)
ON CONFLICT(server_id, ts) DO UPDATE SET cpu=excluded.cpu, ...;
```

## 6. Kunci `app_settings`

| key | value (JSON) | Default |
|---|---|---|
| `display_tz` | `"Asia/Jakarta"` | ya |
| `quiet_hours` | `{"enabled":false,"start":"22:00","end":"06:00","tz":"Asia/Jakarta"}` | |
| `group_offline_threshold` | `5` | |
| `group_offline_window_s` | `120` | |
| `channel_rate_limit_per_min` | `20` | |
| `ui_thresholds` | `{"warn":70,"crit":90}` | |

## 7. Aturan Penulisan (untuk paket `store`)

- Semua fungsi menerima `context.Context`; query tulis menerima `*sql.Tx`.
- Satu sample = satu transaksi. Gunakan prepared statement di dalam transaksi untuk tabel turunan.
- Jangan pernah memakai `SELECT *`; sebut kolom.
- Tidak ada `fmt.Sprintf` untuk membangun SQL dengan nilai; satu-satunya pengecualian: nama tabel dari *allowlist konstanta* (mis. job retensi).
- Pemetaan baris ke struct dilakukan manual (tanpa ORM).
