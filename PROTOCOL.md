# PROTOCOL.md — Spesifikasi Protokol Agent

> Sumber: audit `agent/luma_agent.sh` v**2.4.2**, `luma_install.sh`, `luma_uninstall.sh`, `luma_update.sh`.
> **Jika dokumen ini bertentangan dengan script, script yang benar.** Perbarui dokumen ini bila agent berubah.

## 1. Endpoint yang Dipanggil Agent

| Pemanggil | Metode + Path | Body | Respons yang diharapkan |
|---|---|---|---|
| `luma_agent.sh` (tiap menit) | `POST /v2/` | `j=<payload>` | Bebas. Agent **membuang respons** (`&> /dev/null`). Backend membalas `200` teks `ok` |
| `luma_install.sh` | `POST /` | `v=install&s=<SID>` | Bebas, dibuang. Balas `200 ok` |
| `luma_uninstall.sh` | `POST /` | `v=uninstall&s=<SID>` | Bebas, dibuang. Balas `200 ok` |
| `luma_install.sh`, `luma_update.sh` | `GET`/`HEAD /agent/...` | - | File agent (lihat `DEPLOYMENT.md`) |

- `Content-Type` = `application/x-www-form-urlencoded` (default `wget --post-file` / `--post-data`). Gunakan `r.ParseForm()`.
- Agent mencoba ulang lewat `wget -t 3 --retry-connrefused`. Kegagalan hanya tampak sebagai server `offline` di dashboard, maka backend **wajib mencatat penolakan** (`ingest_rejections`).
- Pola ServeMux Go: `POST /v2/{$}` dan `POST /{$}` (hanya path persis).
- Membedakan `POST /`: ada field `v` dan `s` → event; selain itu `400`.

## 2. Amplop Payload (`j`)

Urutan di sisi agent: `JSON → gzip → base64 (satu baris) → ganti "/" jadi "%2F" dan "+" jadi "%2B"`. Body: `j=<hasil>`.

Urutan decode di backend:

1. `ParseForm()` (otomatis meng-URL-decode `%2F`, `%2B`, dan `=` bila ada) → `j`.
2. `base64.StdEncoding.DecodeString(j)`.
3. `gzip.NewReader` + `io.LimitReader(LUMA_INGEST_MAX_DECOMPRESSED + 1)`; jika melebihi batas → `413`.
4. `json.Unmarshal` ke `map[string]string`.

> **Semua nilai JSON bertipe string**, termasuk angka (`"cpu":"12.5"`). Jangan memakai struct dengan tipe numerik untuk decode awal.

## 3. Field Skalar (bukan base64)

| Field | Tipe setelah parse | Arti | Catatan |
|---|---|---|---|
| `version` | string | Versi agent (mis. `2.4.2`) | simpan ke `servers.agent_version` |
| `SID` | string (32) | Server ID | **jangan dilog**; hash lalu cocokkan |
| `agent` | string | selalu `"0"` | abaikan |
| `user` | string | user yang menjalankan agent (`root`/`luma`) | simpan `servers.agent_user` |
| `reqreboot` | `0`/`1` | butuh reboot | |
| `uptime` | int | detik | |
| `cpusockets` | int | jumlah socket | bisa `0` pada VM |
| `cpucores` | int | **jumlah CPU logis** (`lscpu` baris `CPU(s):`), bukan core fisik | simpan sebagai `cpu_logical` |
| `cputhreads` | int | thread per core | |
| `cpuspeed` | int | MHz rata-rata | |
| `cpu` | float | % pemakaian CPU total (100 - idle), rata-rata 1 menit | |
| `us`, `sy`, `wa`, `st` | float | % user, system, IO wait, steal | |
| `load1`, `load5`, `load15` | float | load average (rata-rata sampel) | |
| `ramsize` | int | total RAM dalam **kB** | |
| `ram` | float | % RAM terpakai, dari `MemAvailable` bila ada | |
| `ramswapsize` | int | total swap dalam kB | `0` bila tanpa swap |
| `ramswap` | float | % swap terpakai | |
| `rambuff`, `ramcache` | float | % buffers / cache yang dapat direklamasi | |

Validasi: persentase di luar `[0, 100]` → dijepit (`clamp`) dan hitung `parse_warnings`; string non-numerik → `NULL`.

## 4. Field Base64

Decode setiap field dengan `base64.StdEncoding` (hasil `base64 | tr -d '\n\r\t '`, jadi tanpa newline). **String kosong = fitur tidak aktif/tidak ada data**, bukan error.

### 4.1 Teks biasa

| Field | Isi | Contoh |
|---|---|---|
| `os` | nama OS | `Ubuntu 22.04.4 LTS` |
| `kernel` | `uname -r` | `5.15.0-107-generic` |
| `hostname` | `uname -n` | `web-01` |
| `cpumodel` | model CPU | `Intel(R) Xeon(R) CPU E5-2680 v4 @ 2.40GHz` |
| `time` | `YYYY-MM-DD HH:MM:SS TZ` (zona lokal server) | **tidak dipakai** untuk `ts` |

### 4.2 Daftar `a,b,c;` (entri dipisah `;`, kolom dipisah `,`)

Cara parse umum: `strings.Split(s, ";")`, buang entri kosong (akhir string berakhir `;`), `strings.Split(entry, ",")`, validasi **jumlah kolom persis**. Baris yang tidak valid: lewati dan hitung `parse_warnings` (jangan gagalkan sample).

| Field | Kolom | Satuan / catatan |
|---|---|---|
| `disks` | `mount,fstype,total,used,avail` | bytes (`df -TPB1`). Mount dengan spasi/koma merusak format → lewati. Duplikat mount: yang terakhir menang. Termasuk entri thin LVM semu dengan `fstype=lvm` |
| `inodes` | `mount,total,used,free` | jumlah inode (`df -Ti`). Nilai `-` (mis. btrfs) → `NULL` |
| `iops` | `key,read_Bps,write_Bps` | bytes/detik. `key` = mount point (untuk ZFS = mountpoint pool) |
| `nics` | `iface,rx_Bps,tx_Bps` | bytes/detik, rata-rata 1 menit |
| `ipv4` | `iface,ip1,ip2,...` | satu entri per interface, jumlah kolom bervariasi (`>= 1`) |
| `ipv6` | `iface,ip1,...` | hanya alamat `global` |
| `conn` | `port,count` | rata-rata koneksi per port (dibulatkan) |
| `temp` | `name,milli_celsius` | **bagi 1000** untuk °C. `name` dapat memuat `\|` (mis. `coretemp-isa-0000\|Core_0`). Nilai `0` persis = tidak terbaca (agent menulis `0` saat gagal) → abaikan. Buang jika di luar `[-100, 200]` °C |
| `serv` | `name,flag` | `1` berjalan, `0` mati |
| `oping` | `name,target,loss,rtt` | lihat 4.3 |

### 4.3 `oping` (ping keluar)

Entri: `TargetName,Target,PacketLossPercent,AvgRTT;`

| Jenis | Ciri `Target` | Satuan `AvgRTT` |
|---|---|---|
| **ICMP** | host/IP biasa | **mikrodetik** (rata-rata ms × 1000); `0` bila tidak ada balasan |
| **TCP** | berakhiran `_<port>` (regex `_\d{1,5}$`), mis. `1.0.0.1_443` | **milidetik** |

Normalisasi wajib: simpan `proto` (`icmp`/`tcp`), pisahkan `target` dan `port`, dan simpan **`rtt_us`** (TCP: `rtt_ms × 1000`). `loss` adalah persen bulat. Ambiguitas hostname ICMP yang berakhiran `_<angka>` diabaikan (sangat jarang).

### 4.4 Teks hardware (semi-terstruktur)

| Field | Format | Parse |
|---|---|---|
| `raid` | `mount,device,<keluaran mdadm -D mentah>;` ... | Pisahkan **maksimal 2 koma pertama** (`SplitN(entry, ",", 3)`). Simpan teks mentah. Turunkan `degraded=true` jika baris `State :` mengandung `degraded` atau `FAILED` |
| `zp` | `mount,pool,<base64 dari zpool status>;` | Pisahkan 3 kolom; kolom 3 di-decode base64 lagi. Turunkan health dari baris `state:` (mis. `ONLINE`, `DEGRADED`, `FAULTED`) |
| `dh` | `type,device,<base64 health>,model,serial;` | `type`: `1` SMART, `2` NVMe, `3` SAS. Kolom 3 di-decode base64 lagi (teks `smartctl`/`nvme`). Turunan status: lihat di bawah |
| `cust` | isi `custom_variables.json` | simpan mentah bila ada (opsional, admin saja) |
| `rps1`, `rps2` | base64 keluaran `ps` | hanya diproses jika fitur `RunningProcesses` dipakai (fase lanjut); **jangan** disimpan default |

Turunan status drive (best-effort, selalu simpan teks mentah juga):

| type | ok | fail |
|---|---|---|
| 1 SMART | teks mengandung `PASSED` | mengandung `FAILED` |
| 2 NVMe | `critical_warning` bernilai `0` | `critical_warning` ≠ `0` |
| 3 SAS | `SMART Health Status: OK` | selain itu |
| lainnya / tidak cocok | - | `unknown` (jangan memicu alert) |

## 5. Yang TIDAK Dikirim Agent

- Tidak ada timestamp epoch → `ts = floor(waktu_terima_UTC / 60) × 60`.
- Tidak ada nama server → nama berasal dari UI; `hostname` hanya informasi.
- Tidak ada header autentikasi → SID di dalam payload satu-satunya kredensial.

## 6. Fakta Perilaku Agent

| Hal | Nilai |
|---|---|
| Jadwal | cron `* * * * *` (atau systemd timer) sebagai `root` atau `luma` |
| Durasi | ±60 detik: mengumpulkan sampel tiap `CollectEveryXSeconds` (default 3) sampai menit berganti, lalu POST |
| Instalasi | `/etc/luma/` (`luma_agent.sh`, `luma.cfg`, log `luma_cron.log`, `debug.log`, `luma_agent.log`) |
| `luma_agent.log` | berisi persis body POST (`j=...`) → **sumber fixture** |
| Proses menggantung | ≥ 10 proses agent berumur ≥ 90 dtk dibunuh; ≥ 50 proses dibunuh semua |
| Fitur opsional (default mati) | `CheckServices`, `CheckSoftRAID`, `CheckDriveHealth`, `RunningProcesses`, `OutgoingPings`, `CustomVars` |
| Perlu root | `CheckSoftRAID`, `CheckDriveHealth` |
| Ketergantungan opsional | `smartmontools`, `nvme-cli`, `lm-sensors`, `ipmitool`, `mdadm`, `zfs` |

Interval data masuk per server ≈ 60 detik dengan jitter beberapa detik. Ambang offline default **180 detik**.

## 7. Event Install / Uninstall

Body form: `v=install&s=<SID>` atau `v=uninstall&s=<SID>`.

| Event | Tindakan backend |
|---|---|
| `install` | Cari server dari hash SID. Set `status='pending'`, tulis `server_events(type='install')`. SID tidak dikenal → `403` + catat |
| `uninstall` | Set `status='decommissioned'`, resolve event alert terbuka tanpa notifikasi, tulis `server_events(type='uninstall')` |

Selalu balas `200 ok` untuk SID yang dikenal. Rate limit sama dengan ingest.

## 8. Contoh Ilustratif (bukan data nyata)

JSON setelah decode awal (nilai base64 dipersingkat untuk keterbacaan; di dunia nyata berupa string base64):

```json
{
  "version": "2.4.2", "SID": "0123456789abcdef0123456789abcdef", "agent": "0", "user": "luma",
  "os": "<b64: Ubuntu 22.04.4 LTS>", "kernel": "<b64: 5.15.0-107-generic>", "hostname": "<b64: web-01>",
  "time": "<b64: 2026-11-14 14:02:03 WIB>", "reqreboot": "0", "uptime": "1234567",
  "cpumodel": "<b64>", "cpusockets": "1", "cpucores": "4", "cputhreads": "2", "cpuspeed": "2400",
  "cpu": "12.5", "wa": "0.4", "st": "0", "us": "9.1", "sy": "3.0",
  "load1": "0.35", "load5": "0.40", "load15": "0.38",
  "ramsize": "8123456", "ram": "41.2", "ramswapsize": "2097148", "ramswap": "0", "rambuff": "1.1", "ramcache": "18.7",
  "disks": "<b64: /,ext4,53687091200,21474836480,29360128000;>",
  "inodes": "<b64: /,3276800,150000,3126800;>",
  "iops": "<b64: /,1024,20480;>",
  "nics": "<b64: eth0,12000,34000;>", "ipv4": "<b64: eth0,203.0.113.10;>", "ipv6": "<b64: eth0,;>",
  "conn": "<b64: 22,1;80,12;443,48;>", "temp": "<b64: x86_pkg_temp,45000;>",
  "serv": "<b64: nginx,1;mysql,1;>", "oping": "<b64: Google,8.8.8.8,0,12500;CFHTTPS,1.0.0.1_443,0,14;>",
  "raid": "", "zp": "", "dh": "", "cust": "", "rps1": "", "rps2": ""
}
```

Setelah parse (ringkas): `disks=[{mount:"/",fstype:"ext4",total:53687091200,used:21474836480,avail:29360128000}]`, `temp=[{sensor:"x86_pkg_temp",celsius:45.0}]`, `oping=[{name:"Google",target:"8.8.8.8",proto:"icmp",loss:0,rtt_us:12500},{name:"CFHTTPS",target:"1.0.0.1",port:443,proto:"tcp",loss:0,rtt_us:14000}]`.

## 9. Fixture Nyata (wajib sebelum parser dianggap selesai)

1. Pasang agent di server uji dengan `DEBUG=1` di `luma.cfg`.
2. Ambil `/etc/luma/luma_agent.log` (body POST persis) → simpan sebagai `backend/testdata/payloads/<nama>.body`.
3. Kumpulkan variasi: Debian/Ubuntu, RHEL/Alma, tanpa swap, ZFS, `mdadm` RAID, NVMe, MegaRAID/SAS, `OutgoingPings` ICMP+TCP, `CheckServices` aktif.
4. **Anonimkan** `SID`, `hostname`, IP sebelum di-commit (ganti SID dengan `0123456789abcdef0123456789abcdef`).
5. Replay lokal: `curl --data-binary @file.body http://localhost:8080/v2/` (`make replay FILE=...`).
6. Debug.log memuat JSON mentah (`JSON:` setelah timestamp) untuk membantu menulis *golden file*.
