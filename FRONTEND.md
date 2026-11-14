# FRONTEND.md — Spesifikasi Frontend

> Stack terkunci: React 18 + Vite + TypeScript (`strict`) + Tailwind + shadcn/ui + TanStack Query + React Router 6 + Zustand + react-hook-form + zod + Recharts + lucide-react + cmdk.
> Kontrak data: `API.md`. Bahasa UI: **Indonesia**.

## 1. Struktur Folder

```
frontend/src/
├── app/            # main.tsx, router.tsx, providers.tsx, layout (AppShell, Sidebar, Topbar)
├── components/
│   ├── ui/         # hasil `npx shadcn add ...` (jangan diedit selain tema)
│   ├── charts/     # TimeSeriesChart, Sparkline, UsageBar, RangeSelector
│   └── common/     # StatusBadge, SeverityBadge, EmptyState, ErrorState, ConfirmDialog, CopyField
├── features/
│   ├── auth/  overview/  servers/  alerts/  manage/  settings/
│   └── (tiap fitur: pages/, components/, api.ts [hooks TanStack Query], schemas.ts)
├── lib/            # api.ts (fetch wrapper), types.ts, format.ts, time.ts, i18n/id.ts, thresholds.ts
├── hooks/          # useTimeRange, useDebounce, useCommandPalette
└── store/          # ui.ts (Zustand: tema, rentang waktu, sidebar)
```

Aturan: komponen tidak memanggil `fetch` langsung; semua data lewat hook di `features/*/api.ts`. Tipe di `lib/types.ts` mencerminkan `API.md` (ditulis manual + validasi `zod` di mode dev).

## 2. Rute

| Path | Halaman | Peran |
|---|---|---|
| `/login` | Login (email, password, TOTP bila perlu) | publik |
| `/` | Overview | viewer |
| `/servers/:id` | Detail server (tab: Ringkasan, Jaringan & Disk, Layanan & Perangkat, Alert, Event) | viewer |
| `/alerts` | Alert aktif dan riwayat | viewer |
| `/manage/servers` | Kelola server, grup, perintah instalasi | admin |
| `/settings/rules` | Aturan alert | admin |
| `/settings/notifications` | Kanal, log pengiriman, preferensi | admin |
| `/settings/users` | Pengguna | admin |
| `/settings/general` | Zona waktu, ambang warna, quiet hours, pengelompokan | admin |
| `/settings/audit` | Audit log | admin |

Guard: tanpa sesi → `/login?next=<path>`. `403` dari API → toast "Tidak punya akses". Rute admin disembunyikan dari sidebar untuk `viewer`.

## 3. Data Fetching

| Hal | Aturan |
|---|---|
| Klien | `lib/api.ts`: `fetch` dengan `credentials:'same-origin'`, `Content-Type: application/json`, parsing error ke `ApiError{code,message,fields}` |
| Query key | `['overview']`, `['servers', params]`, `['server', id]`, `['server', id, 'metrics', rangeKey]`, dst. |
| Polling | Overview 15 dtk; Server Detail 30 dtk; Alerts 15 dtk; `refetchIntervalInBackground:false` |
| Cache | `staleTime` 10 dtk; `retry: 1` |
| Mutasi | `useMutation` + invalidasi key terkait + toast sukses/gagal |
| `401` | bersihkan cache, redirect ke login |
| Rentang waktu | state global (`store/ui.ts`): `1h|6h|24h|7d|30d`; mengirim `range=` ke API |

## 4. Halaman

### 4.1 Overview

- **Kartu KPI** (dari `/api/overview`): Total server, Online, Offline, Alert aktif (dengan rincian severity), Rata-rata CPU, Rata-rata RAM.
- **Toolbar**: pencarian, filter status, filter grup, urutkan.
- **Tabel server** (TanStack Table): Nama + grup, Status (badge), CPU/RAM/Disk (`UsageBar`), Sparkline CPU, Load, Uptime, "Terakhir terlihat", jumlah alert. Klik baris → `/servers/:id`.
- Status `offline` baris diberi penekanan visual (ikon + teks, bukan hanya warna). `pending` = abu-abu "Menunggu data". `decommissioned` = redup "Dilepas". Badge "Maintenance" jika `maintenance=true`.
- State: skeleton saat loading, `EmptyState` ("Belum ada server, tambahkan di Kelola Server") bila kosong, `ErrorState` dengan tombol ulangi.

### 4.2 Detail Server

Header: nama, status, hostname, OS/kernel, uptime, IP, versi agent (badge "usang" bila berbeda dari versi agent di backend), tombol Maintenance (admin).

| Tab | Isi |
|---|---|
| **Ringkasan** | Grafik CPU (stacked area us/sy/wa/st), Load (1/5/15), RAM (used, buffers, cache), Swap |
| **Jaringan & Disk** | Trafik per interface (rx/tx), I/O disk (read/write per mount), tabel disk (mount, fstype, `UsageBar`, total/terpakai/sisa, inode %), koneksi per port |
| **Layanan & Perangkat** | Daftar layanan (hijau/merah), suhu (grafik + nilai terbaru), ping keluar (loss dan RTT), RAID/ZFS/drive (status + teks mentah di `<pre>` untuk admin) |
| **Alert** | Alert server ini (aktif dan riwayat) |
| **Event** | Timeline `server_events` |

Semua grafik memakai `RangeSelector` yang sama dan **crosshair sinkron** (`syncId` Recharts).

### 4.3 Alerts

Dua tab: **Aktif** (firing + pending) dan **Riwayat**. Kolom: severity, server, aturan, subjek, nilai, mulai, durasi, status ack. Aksi admin: **Acknowledge**. Filter: severity, server, rentang waktu.

### 4.4 Kelola Server (admin)

- Tabel server + grup; dialog **Tambah Server** (nama, grup, "Jalankan sebagai root" dengan peringatan bahwa RAID dan drive health butuh root).
- Setelah dibuat: dialog menampilkan **SID sekali** dan perintah `install` (komponen `CopyField`) dengan peringatan "SID tidak akan ditampilkan lagi". Tombol **Rotate SID**, **Maintenance**, **Hapus** (dengan `ConfirmDialog`).
- Menu **Salin perintah update / uninstall** per server.

### 4.5 Pengaturan

- **Aturan**: tabel + form (react-hook-form + zod) sesuai `Rule`; pilihan kanal multi-pilih; pratinjau kalimat ("CPU > 90% selama 10 menit → warning").
- **Notifikasi**: daftar kanal, form Telegram (token, chat id, tombol **Deteksi Chat**, **Kirim tes**), form Email (SMTP, penerima, **Kirim tes**), log pengiriman dengan tombol **Kirim ulang**. Field secret menampilkan `********`; hanya terkirim bila diubah.
- **Umum**: zona waktu tampilan, ambang warna, quiet hours, ambang pengelompokan offline.
- **Pengguna**, **Audit**: tabel dan form sederhana.

## 5. Spesifikasi Grafik

| Grafik | Endpoint | Seri | Satuan | Sumbu Y |
|---|---|---|---|---|
| CPU | `/metrics` | `cpu_us`, `cpu_sy`, `cpu_wa`, `cpu_st` (stack) | % | 0-100 |
| Load | `/metrics` | `load1`, `load5`, `load15` | - | auto, min 0 |
| RAM | `/metrics` | `ram`, `ram_buff`, `ram_cache` | % | 0-100 |
| Swap | `/metrics` | `swap` | % | 0-100 |
| Jaringan | `/network` | `rx`, `tx` per iface | **bit/detik** (bytes × 8) → `Kbps/Mbps/Gbps` | auto |
| I/O disk | `/disk-io` | `read`, `write` per mount | bytes/detik → `KB/s, MB/s` | auto |
| Suhu | `/temps` | satu garis per sensor | °C | auto |
| Ping | `/ping` | `rtt_ms` (garis), `loss` (area/batang) per target | ms, % | auto |
| Koneksi | `/connections` | satu garis per port | jumlah | auto |

Aturan:
- Respons berkunci (`series`) digabung menjadi array datar per `ts` di `lib/format.ts` (`mergeSeries`), nilai hilang = `null` dan garis **tidak disambung** (`connectNulls=false`).
- Titik ≤ ~500 per seri (diatur backend). Tooltip menampilkan waktu dalam `display_tz`.
- Jika `source:"hourly"`, tampilkan label "Rata-rata per jam" dan (bila ada) pita `*_max`.
- Warna seri berasal dari token CSS (`--chart-1..8`) sehingga mendukung dark/light mode. Jangan hardcode hex di komponen.

## 6. Pemformatan (`lib/format.ts`)

| Fungsi | Perilaku |
|---|---|
| `formatBytes(n)` | basis 1024, satuan `B, KB, MB, GB, TB`, 1-2 desimal |
| `formatBitsPerSec(bytesPerSec)` | ×8, satuan `bps, Kbps, Mbps, Gbps` (basis 1000) |
| `formatPercent(n)` | 1 desimal, `—` bila `null` |
| `formatDuration(sec)` | `3h 12m`, `5d 2h`, `45s` |
| `formatRelative(ts)` | "12 detik lalu", "3 menit lalu" |
| `formatDateTime(ts, tz)` | `Intl.DateTimeFormat('id-ID', {timeZone})` |

Teks UI dipusatkan di `lib/i18n/id.ts` (objek sederhana; tanpa library i18n).

## 7. Ambang Warna (`lib/thresholds.ts`)

Default dari `ui_thresholds` (`warn` 70, `crit` 90): `< warn` hijau, `warn..crit` kuning, `≥ crit` merah. Disk memakai ambang yang sama. Status **tidak** hanya dibedakan lewat warna: sertakan ikon (`CircleCheck`, `TriangleAlert`, `OctagonAlert`) dan teks aksesibel.

## 8. UX dan Aksesibilitas

- Dark/light mode mengikuti sistem, dapat diganti; disimpan di Zustand (+ `localStorage` untuk **preferensi UI saja**, bukan token).
- Command palette `Ctrl/Cmd+K` (`cmdk`): lompat ke server, halaman, dan aksi cepat.
- Responsif: sidebar menjadi drawer di < 1024 px; tabel menjadi kartu di < 640 px; grafik tinggi minimal 200 px.
- Kontras memenuhi WCAG AA; fokus keyboard terlihat; dialog menjebak fokus (bawaan Radix).
- Indikator data usang: bila `now - last_seen > 180 dtk`, tampilkan banner "Data terakhir X lalu".
- Salin ke clipboard memberi toast "Disalin".

## 9. Build dan Integrasi

- Dev: `vite` di `:5173` dengan proxy `/api`, `/healthz` ke `http://localhost:8080`.
- Prod: `npm run build` → `frontend/dist` → disalin ke `backend/internal/web/dist` saat build Docker; di-*embed* dengan `//go:embed`. Repo menyimpan placeholder `backend/internal/web/dist/index.html` agar `go build` tidak gagal sebelum frontend dibangun.
- Backend melayani file statis dari embed dan **fallback ke `index.html`** untuk path non-`/api` yang tidak cocok file (`GET`), dengan `Cache-Control: no-cache` untuk `index.html` dan `immutable` untuk `/assets/*`.
- Perintah shadcn dijalankan sekali pada T-033: `npx shadcn@latest init` lalu `add` komponen yang dipakai (button, card, table, dialog, dropdown-menu, select, switch, tabs, toast/sonner, form, input, badge, skeleton, command, sheet, tooltip).

## 10. Pengujian Frontend

- Vitest + Testing Library untuk: `format.ts`, `mergeSeries`, `thresholds`, komponen `UsageBar`, `StatusBadge`, form aturan (validasi zod).
- Mock API dengan `msw` hanya untuk tes (dev dependency).
- E2E (Playwright, fase 10): login, tambah server → muncul perintah instalasi, buat aturan, tes kirim kanal (dengan backend uji).
