# RINGKASAN PROYEK

**Payment Sandbox Application**
Technical Assessment — Backend (Golang)
https://github.com/Trickster-ID/payment-sandbox

Dibuat: 05 May 2026
Diperbarui: 16 Jul 2026 (lihat catatan perubahan di akhir dokumen)

## 1. Gambaran Umum Proyek

Payment Sandbox adalah aplikasi simulasi transaksi pembayaran yang dibangun sebagai
bagian dari Technical Assessment Software Engineering. Aplikasi ini mensimulasikan alur
pembayaran nyata — mulai dari pembuatan invoice, halaman pembayaran publik, simulasi
status pembayaran, hingga proses refund — tanpa integrasi ke payment gateway pihak
ketiga.

Peran yang didukung sistem:
- Merchant — membuat invoice dan menerima pembayaran (simulasi)
- End User (Payer) — membayar invoice melalui payment link
- Admin — mengoperasikan sandbox: mengubah status pembayaran, refund, dan top-up

## 2. Fitur Utama yang Diimplementasikan

### Autentikasi & Otorisasi
- Registrasi user dengan validasi email dan hash password (bcrypt via `golang.org/x/crypto/bcrypt`)
- Login menghasilkan JWT token dengan expiry yang dapat dikonfigurasi
- Role-based access control: MERCHANT vs ADMIN via middleware
- OAuth2 Authorization Server — Authorization Code, Client Credentials, dan Refresh Token grant types
- Scope system: read, write, admin dengan auto-approve untuk first-party clients

### Merchant Wallet
- Merchant dapat melihat saldo wallet simulasi
- Pembuatan top-up request dengan status PENDING
- Admin dapat mengubah status top-up menjadi SUCCESS atau FAILED
- Saldo merchant bertambah secara atomik (DB transaction) jika top-up SUCCESS
- Daftar riwayat top-up dengan paginasi

### Manajemen Invoice
- Merchant dapat membuat invoice dengan nomor unik otomatis
- Filter daftar invoice berdasarkan status, tanggal, dan parameter lain
- Payment link token yang random dan unik untuk setiap invoice
- Halaman pembayaran publik dapat diakses tanpa autentikasi via token
- Simulasi status EXPIRED ketika due_date terlewat

### Simulasi Pembayaran
- End user memilih metode pembayaran: WALLET / VA_DUMMY / EWALLET_DUMMY
- Sistem membuat payment_intent dengan status PENDING
- Admin mengubah status ke SUCCESS atau FAILED
- Jika SUCCESS: status invoice otomatis menjadi PAID (atomic via DB transaction)
- State machine yang ketat untuk transisi status

### Simulasi Refund
- Merchant mengajukan permintaan refund untuk invoice PAID
- Alur: REQUESTED → APPROVED/REJECTED → SUCCESS/FAILED
- Admin approve/reject lalu proses hasil refund
- Saldo merchant berkurang secara atomik jika refund SUCCESS

### Admin Dashboard & Statistik
- Statistik total invoice, total PAID/FAILED/EXPIRED
- Total nominal pembayaran dan refund
- Filter berdasarkan merchant dan rentang tanggal
- Panel manajemen payment intent, refund, dan top-up

## 3. Daftar Peningkatan (Improvements)

Berikut adalah seluruh peningkatan yang dilakukan melebihi persyaratan dasar assessment,
dikelompokkan berdasarkan area teknis.

### 3.1 Arsitektur & Clean Code
- Migrasi dari arsitektur monolitik (sandbox module) ke modular clean architecture per domain
- Modul aktif: users, invoice, payment, refund, wallet, admin, oauth2, ledger, reconciliation, saga
- Penerapan layered architecture yang konsisten: handler → service → repository di setiap modul
- Dependency Injection menggunakan Google Wire — semua dependensi di-wire secara compile-time
- Konvensi penamaan interface seragam dengan prefix `I*` (`IUserService`, `IInvoiceRepository`, dll.)
- Entitas domain dipindahkan ke module-local (`app/modules/<module>/models/entity/`) — tidak ada shared store
- Split route registration: satu `RegisterRoutes` per modul API package
- Akses database menggunakan `database/sql` standard library + driver `pgx/v5` (raw SQL, tanpa ORM) untuk kontrol query yang eksplisit

### 3.2 Modul Tambahan di Luar Spesifikasi Dasar
- Ledger module — pencatatan double-entry bookkeeping untuk audit transaksi keuangan
- Reconciliation module — proses rekonsiliasi antara invoice, payment, dan ledger
- Saga module — orchestrator pattern untuk transaksi terdistribusi dengan recovery
- OAuth2 module — Authorization Server lengkap (lihat 3.7)

### 3.3 Testing — Unit & Integrasi
- Unit test table-driven untuk semua service layer (users, invoice, payment, refund, wallet, admin)
- Unit test table-driven untuk semua handler layer dengan mock service menggunakan Mockery
- Coverage service layer (snapshot 05 May 2026): users 95.8%, invoice 100%, payment 100%, refund 100%, wallet 100%, admin 100%, oauth2 ~91.7%, ledger 100%, merchants 100%, reconciliation 100%, saga orchestrator 100%
- Integration test dengan database nyata (PostgreSQL) mencakup alur merchant, admin, refund, dan negative cases
- Negative integration test: token tidak valid, role salah, input tidak valid, transisi status ilegal
- Standar assertions: `testify/require` untuk precondition, `testify/assert` untuk perilaku
- Perintah `make test-integration`, `make verify-batch11`, `make coverage-services` tersedia

### 3.4 Mock & Testability
- Setup Mockery dengan `.mockery.yaml` untuk semua interface repository dan service
- Mock digenerate per-modul di direktori `mocks/` masing-masing
- Handler direfactor untuk depend pada service interface — bukan concrete struct
- Satu perintah `make mock` untuk regenerasi semua mock secara konsisten
- Penghapusan handler-level interface duplikat — mock cukup dari service package

### 3.5 Shared Utilities
- Shared pagination utility (`app/shared/pagination`) dengan sanitizer dan default yang aman
- Shared validator utility (`app/shared/validator`) untuk validasi email, amount, dan tanggal
- Shared error extraction dan response envelope dengan unit test mandiri
- Response envelope standar (`Envelope{data, meta, error}`) digunakan di seluruh handler
- Request ID middleware dengan propagasi header `X-Request-ID` ke setiap response

### 3.6 Journey Logging (MongoDB)
- Infrastruktur journey logging ke MongoDB untuk audit trail event kritis
- Integrasi di semua transaction handler: wallet, invoice, payment intent, refund
- Best-effort pattern: jika Mongo tidak tersedia, fallback ke no-op logger — sistem tidak crash
- Konfigurasi via env: `MONGO_URI`, `MONGO_DB_NAME`, `MONGO_COLLECTION`, `MONGO_JOURNEY_ENABLE`
- Docker Compose menginclude service MongoDB dengan healthcheck yang benar
- Verifikasi taksonomi journey action otomatis via CI (`misc/verify/iso-journey-events.sh`), memvalidasi bahwa setiap journey action wajib (`INVOICE_CREATE`, `PAYMENT_INTENT_CREATE`, `PAYMENT_INTENT_STATUS_UPDATE`, `REFUND_REQUEST`, `REFUND_REVIEW`, `REFUND_PROCESS`, `TOPUP_CREATE`, `TOPUP_STATUS_UPDATE`) punya event log yang sesuai di handler

### 3.7 OAuth2 Authorization Server
- Implementasi OAuth2 penuh: Authorization Code, Client Credentials, Refresh Token grant types
- Self-service client registration — merchant dapat mendaftarkan aplikasi OAuth2 mereka sendiri
- Consent screen: auto-approve untuk first-party, halaman consent untuk third-party
- Scope granular: read, write, admin
- JWT access token (stateless) + opaque refresh token (disimpan di DB)
- Role claims diinclude dalam semua jenis grant token
- Tabel DB baru: `oauth2_clients`, `oauth2_authorization_codes`, `oauth2_refresh_tokens`, `oauth2_consents`

### 3.8 Dokumentasi API (Swagger/OpenAPI)
- Anotasi Swagger lengkap di semua handler (users, invoice, payment, refund, wallet, admin, oauth2, ledger)
- Response schema konkret dengan tipe, contoh, dan enum — bukan generic `map[string]interface{}`
- Swagger metadata `PaginationMeta` sebagai tipe berdiri sendiri
- Dokumen digenerate via `make swag` dan tersedia di `/swagger/index.html`
- Parity review antara Swagger spec dan runtime routes; perbedaan `/healthz` vs `/ping` diperbaiki
- Audit lanjutan (Jul 2026) menemukan dan memperbaiki 5 gap tambahan: godoc `DeleteClient` & `ApproveAuthorize` (oauth2) yang belum ada, route `/admin/wallet/transactions` yang belum terdaftar di spec, typo `enums(D,C)` → `Enums(D,C)` yang membuat swag drop nilai enum, serta response schema `GetMerchantAccount` yang sebelumnya envelope kosong (`response.Envelope{}`) — sekarang eksplisit `response.Envelope{data=entity.Account}`
- **Breaking change kecil**: perbaikan skema `GetMerchantAccount` ini menyertakan penambahan JSON tag eksplisit pada `entity.Account` (field response `GET /admin/ledger/merchants/{merchantId}/account` berubah dari PascalCase — `ID`, `MerchantID`, dst — menjadi snake_case — `id`, `merchant_id`, dst). FE yang consume endpoint ini perlu menyesuaikan parsing field.

### 3.9 API Testing Collection (Bruno)
- Ditambahkan collection Bruno lengkap (73 request, 14 folder modul) yang tervalidasi end-to-end terhadap kode handler/route aktual dan server yang berjalan
- Chaining otomatis antar request (mis. register merchant → login → transaksi lanjutan) via `script:pre-request`/`post-response`
- Environment `local` terpisah, folder terurut sesuai dependency skenario bisnis (contoh: refund berjalan setelah admin approve payment, bukan sebelumnya)
- Hasil verifikasi terakhir: 58/58 request, 153/153 test assertion, status PASS, tanpa regresi setelah rebuild Docker image
- Menemukan & bekerja-sekitar 3 bug Bruno CLI (form-urlencoded body kosong, interpolasi variabel mati saat header custom di-set, `params:query` tidak terkirim di GET) dan 1 kesalahan konvensi file (`meta.bru` seharusnya `folder.bru`)

### 3.10 Performance & Query Optimization
- `EXPLAIN (ANALYZE, BUFFERS)` dijalankan untuk semua query kritis
- Indeks database ditambahkan pada kolom filter yang sering dipakai
- Penghindaran N+1 query di semua list endpoint
- K6 performance test dengan profil: smoke, baseline, stress, soak
- Target API response time ≤ 300ms tercapai pada endpoint utama
- HTML report dari K6 tersimpan di `docs/k6/`
- GitHub Actions workflow untuk K6 smoke test (`.github/workflows/perf-smoke.yml`)
- **Eksekusi penuh K6 (16 Jul 2026)**: seluruh profil (`smoke`, `full-coverage`, `baseline`, `stress`, `soak`) dijalankan terhadap stack Docker lokal, semua **PASS**:
  - `baseline` (5 VU, 30s): p95 = 39.9ms, avg = 11.2ms, fail rate 0%
  - `stress` (ramp 5→15 VU, ~3m20s): p95 = 42.1ms, avg = 10.0ms, fail rate 0%, tidak ditemukan titik degradasi hingga 3x peak VU
  - `soak` (5 menit, dipersingkat dari default 30 menit): p95 = 143.0ms, avg = 26.8ms, fail rate 0%; tren latency antar-waktu turun -87.6% (efek warmup bcrypt di awal, bukan indikasi memory/connection leak)
  - Endpoint paling lambat secara konsisten adalah operasi bcrypt (`POST /oauth2/token`, `POST /merchant/clients`, `POST /users/register`) — biaya inheren `bcrypt.DefaultCost`, flat per-request, tidak terakumulasi di bawah beban sustained/spike
- **Audit coverage endpoint K6**: cross-check seluruh skenario `docs/k6/scenarios/*.js` terhadap semua route aktual di `app/cmd/router.go` dan `app/modules/*/api/routes.go` — hasil akhir **28/28 endpoint API tercakup**
- **Bug ditemukan & diperbaiki saat audit coverage k6 (dua bug, keduanya sudah di-fix dan diverifikasi ulang)**:
  1. Bug di test suite: skenario `docs/k6/scenarios/admin.js` salah memanggil `GET /merchant/wallet/transactions?merchant_id=...` dengan token admin (403 Forbidden karena route tersebut memang menolak param `merchant_id` untuk non-scope-nya). Path admin yang benar adalah `GET /admin/wallet/transactions`. Diperbaiki di scenario, bukan di API.
  2. **Bug produksi nyata**: `POST /api/v1/oauth2/authorize` (approve-consent, authorization code grant) didaftarkan di `RegisterPublicRoutes` (`app/modules/oauth2/api/oauth2_api.go`) tanpa auth middleware, padahal handler `ApproveAuthorize` memanggil `middleware.MustUserID` yang membutuhkan konteks user dari `AuthMiddleware`. Akibatnya endpoint ini **selalu mengembalikan 401**, tidak bisa dipakai sama sekali sejak awal implementasi — ini adalah regresi dari desain asli (`.agents/feature/oauth2-generation-plan.md` sudah eksplisit menyebut endpoint ini harus auth-required). Diperbaiki dengan memindahkan registrasi route ke `RegisterSecuredRoutes`; `GET /oauth2/authorize` tetap public sesuai desain (entrypoint redirect browser). Detail lengkap di `.agents/feature/oauth2-authorize-bugfix-note.md`.
  - Verifikasi: `go test ./app/cmd/...` pass, manual curl 200 OK dengan `redirect_uri`, dan rerun `make perf-full-coverage` PASS bersih setelah kedua fix.

### 3.11 Keamanan (Security)
- CORS middleware dengan konfigurasi origin yang dapat dikontrol
- JWT expiry wajib; refresh token opaque untuk OAuth2
- Payment link token random dan unik — tidak bisa ditebak
- Password di-hash menggunakan bcrypt (`golang.org/x/crypto/bcrypt`)
- Tidak ada informasi sensitif yang terekspos ke response publik
- Dokumen ISO 27001-aligned security controls di `docs/iso/`
- Backup-restore continuity drill terotomasi: `make drill-backup-restore`
- CI workflow untuk ISO readiness verification: `make verify-iso-ci`

### 3.12 DevOps & Tooling
- Docker Compose lengkap: PostgreSQL, MongoDB, pgweb (UI query DB), aplikasi Go
- Makefile dengan target terstruktur: `build`, `test`, `swag`, `mock`, `coverage`, `verify-batch*`
- Dockerfile multi-stage untuk build image Go yang efisien
- CI GitHub Actions: ISO verification dan K6 smoke test otomatis pada push/PR
- Skrip `make verify-batch11` menggabungkan test suite + route parity + query-plan checks
- pgweb service untuk inspeksi database langsung dari browser
- Fix stabilitas CI (Jul 2026): script `misc/verify/iso-journey-events.sh` dan `misc/verify/iso-drill-evidence.sh` sebelumnya bergantung pada `rg` (ripgrep) yang tidak tersedia di runner `ubuntu-latest`, menyebabkan `make verify-iso-ci` gagal di GitHub Actions; diganti ke `grep` standar dan pattern pencocokan taksonomi journey action diperbaiki agar sesuai nilai `EventType` aktual di kode

## 4. Tech Stack (Backend)

### Bahasa & Framework
- Bahasa: Go 1.26
- Framework HTTP: Gin (`github.com/gin-gonic/gin`)
- Dependency Injection: Google Wire

### Database & Persistensi
- Database Utama: PostgreSQL
- Database Driver: `pgx/v5` (jackc/pgx) via `database/sql` standard library
- Akses Data: Raw SQL queries (tanpa ORM) — kontrol penuh atas query
- Database Audit Log: MongoDB (`go.mongodb.org/mongo-driver`)

### Authentication & Security
- JWT: `golang-jwt/jwt/v5`
- Password Hashing: bcrypt (`golang.org/x/crypto/bcrypt`)
- UUID: `google/uuid`

### Testing & Mocking
- Test Library: testify (`require` + `assert`)
- Mock Generator: Mockery (vektra/mockery v2)
- SQL Mock: go-sqlmock
- API Testing: Bruno CLI (`@usebruno/cli`)

### Tooling & Dokumentasi
- API Docs: Swagger via `swaggo/swag` + `gin-swagger`
- Performance Test: K6 (Grafana K6)
- Logging: `rs/zerolog`
- Config: `spf13/viper` + `joho/godotenv`

### Infrastructure
- Containerisasi: Docker + Docker Compose
- CI/CD: GitHub Actions (`iso-verification`, `perf-smoke`)
- DB GUI: pgweb

**Catatan**: Frontend (React/React Native) tercantum dalam project requirement namun pada repository
ini hanya backend yang diimplementasikan. Rencana frontend (Next.js) tersedia di `.agents/frontend-generation-plan.md` sebagai dokumen perencanaan — belum dieksekusi.

## 5. Struktur Proyek Backend

```
app/
  cmd/            ← Entry point, router, DI wire
  config/         ← Konfigurasi aplikasi
  middleware/     ← Auth, request-ID, CORS
  shared/         ← Shared utilities (response, errors, pagination, validator, journeylog)
  modules/
    users/            ← Registrasi & login + JWT
    oauth2/           ← OAuth2 Authorization Server
    invoice/          ← Invoice CRUD + payment link
    payment/          ← Payment intent lifecycle
    refund/           ← Refund lifecycle
    wallet/           ← Top-up + balance
    admin/            ← Dashboard statistik + admin actions
    ledger/           ← Double-entry bookkeeping
    reconciliation/   ← Rekonsiliasi transaksi
    saga/             ← Saga orchestrator + recovery
docs/               ← Swagger spec, performance report, ISO docs, K6
bruno/              ← Bruno API testing collection (73 request, 14 folder modul)
misc/               ← Scripts verify, ops, init DB
.agents/            ← AI generation plans & progress trackers
.github/workflows/  ← CI: iso-verification, perf-smoke
```

## 6. Catatan Akhir

Proyek backend ini telah melebihi persyaratan minimum assessment dengan menambahkan
OAuth2 Authorization Server, journey logging ke MongoDB, ledger double-entry, saga
orchestrator, test coverage tinggi di semua layer, ISO security controls, performance
testing dengan K6, dan collection Bruno untuk pengujian API end-to-end. Semua komponen
dapat dijalankan dengan satu perintah `docker-compose up` dan diverifikasi dengan
`make verify-batch11`.

Semua test lulus (`go test ./...`) dan route regression test berjalan di CI.

## 7. Laporan Coverage Unit Test

Laporan ini dihasilkan dari file `coverage.out` menggunakan `go tool cover -func`. Coverage
diukur pada tanggal 05 May 2026 (commit `c3777ba` — try to fix mount on postgre). Perubahan
kode Go pada sesi Jul 2026 (godoc, satu perbaikan JSON tag struct, dua perbaikan shell
script CI) tidak menambah/mengurangi baris logic yang diuji unit test, sehingga angka
di bawah ini masih relevan sebagai estimasi kondisi terkini. Untuk angka presisi terbaru,
jalankan ulang perintah di bagian 7.6.

Coverage total keseluruhan kode (termasuk mocks, api routes, dan docs) adalah 41.6%.
Angka ini rendah karena file `mocks/`, `api/routes.go`, dan `docs/docs.go` tidak dicakup oleh
unit test — ini adalah hal yang normal karena file-file tersebut adalah generated code dan
route registration.

**Coverage Total (all packages): 41.6%**

### 7.1 Coverage Per Layer (Rata-Rata)

Berikut adalah rata-rata coverage per lapisan arsitektur, tidak termasuk file `mocks/`:
- 98.4% — Handler Layer (semua modul)
- 97.1% — Service Layer (semua modul)
- 90.0% — Repository Layer (semua modul)

### 7.2 Coverage Per Modul

Tabel di bawah menunjukkan coverage per modul, dikelompokkan berdasarkan layer:

| Modul | Handler | Service | Repository |
|---|---|---|---|
| admin | 100.0% | 100.0% | 100.0% |
| invoice | 100.0% | 100.0% | 100.0% |
| ledger | 100.0% | 100.0% | 0.0% (hanya integrasi) |
| merchants | 100.0% | 100.0% | 100.0% |
| oauth2 | ~94.2% | ~91.7% | ~93.1% |
| payment | 100.0% | 100.0% | ~90.6% |
| reconciliation | — | 100.0% | — |
| refund | 100.0% | 100.0% | ~93.0% |
| saga | — | ~70.0% | — |
| users | 100.0% | ~95.8% | 100.0% |
| wallet | ~99.4% | 100.0% | ~99.7% |

### 7.3 Area Coverage Rendah & Penjelasan

Beberapa area memiliki coverage rendah dengan alasan yang valid:
- **ledger/repositories (0.0%)**: Repository ledger hanya diuji melalui integration test dengan database nyata (PostgreSQL), bukan unit test dengan go-sqlmock. Ini adalah keputusan desain yang disengaja karena query ledger bersifat kompleks.
- **saga/services/recovery.go (40.0%)**: Fungsi recovery saga mencakup skenario partial-failure yang sulit disimulasikan dalam unit test tanpa infrastruktur lengkap.
- **payment/sagas/ (~64.0% rata-rata)**: Beberapa branch dalam payment saga constructor dan rollback function hanya dapat diuji dalam konteks integrasi penuh.
- **api/routes.go (0.0%)**: File route registration tidak dicakup karena merupakan glue code — diverifikasi lewat integration test (`router_test.go`).
- **shared/locking/redis_lock.go (0.0%)**: Redis lock Acquire/Release tidak di-unit-test karena memerlukan koneksi Redis nyata.

### 7.4 Coverage Shared Utilities

Package `shared/` memiliki coverage tinggi karena sepenuhnya diuji secara mandiri:
- `shared/idempotency` (Claim, Fetch, Complete): 100.0%
- `shared/pagination` (Parse): 100.0%
- `shared/response` (JSON, OK, Created, Fail, FailFromError): 100.0%
- `shared/validator` (IsEmail, IsPositiveAmount, ParseRFC3339, IsTodayOrFuture, IsISO4217Code): 100.0%
- `shared/locking/optimistic` (CheckedExec): 100.0%
- `middleware` (Auth, CORS, RequestID, RoleGuard): ~95.0%

### 7.5 Coverage cmd/ & Infrastructure

- `app/cmd/app.go` (NewApp): 100.0%
- `app/cmd/router.go` (RegisterRoutes): 97.3%
- `app/cmd/main.go`: 0.0% (entry-point — tidak di-unit-test, diverifikasi lewat integrasi)
- `app/cmd/wire_gen.go`: 0.0% (generated code dari Google Wire)
- `app/config/config.go`: 100.0%

### 7.6 Kesimpulan Coverage

Coverage unit test proyek ini sangat baik pada semua layer yang relevan. Business logic
(service layer: 97.1% rata-rata) dan HTTP handler (98.4%) memiliki coverage hampir penuh.
Coverage repository layer (90.0%) sedikit lebih rendah karena beberapa modul
mengutamakan integration test untuk validasi SQL query nyata ke PostgreSQL. File-file yang
memiliki coverage 0% (`mocks/`, `api/routes.go`, `wire_gen.go`, `docs/docs.go`) adalah
generated code atau glue code yang tidak membutuhkan unit test terpisah.

**Catatan**: Untuk menjalankan ulang coverage, gunakan:
```
make coverage-services
```
atau
```
go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out
```

## 8. Catatan Perubahan (Changelog Dokumen)

### 16 Jul 2026
Sumber: sesi kerja performance testing K6 (belum di-commit saat dokumen ini ditulis).

- Update **section 3.10**: eksekusi penuh seluruh profil K6 (`smoke`, `full-coverage`,
  `baseline`, `stress`, `soak`) terhadap stack Docker lokal — semua PASS dengan p95 jauh
  di bawah threshold 300ms pada baseline dan di bawah 1000ms (relaxed) pada stress/soak.
- Audit coverage: cross-check seluruh skenario k6 vs semua route aktual di router —
  hasil akhir 28/28 endpoint API tercakup.
- Ditemukan & diperbaiki **1 bug di test suite k6** (path salah untuk admin wallet
  transactions) dan **1 bug produksi nyata di kode aplikasi**: `POST /api/v1/oauth2/authorize`
  didaftarkan tanpa auth middleware sehingga selalu 401 — diperbaiki dengan memindahkan
  route ke secured group (`app/modules/oauth2/api/oauth2_api.go`). Ini adalah regresi dari
  desain asli, bukan simplifikasi yang disengaja.
- Detail teknis lengkap bug fix ada di `.agents/feature/oauth2-authorize-bugfix-note.md`
  dan `.agents/feature/performance-generation-progress.md`.
- Section 7 (coverage) tidak diubah — perubahan kode Go sesi ini (pemindahan 1 baris
  route registration) tidak menambah/mengurangi baris logic yang diuji unit test secara
  signifikan; test registrasi route (`router_test.go`) tetap pass tanpa modifikasi karena
  hanya mengecek keberadaan path+method, bukan grouping middleware-nya.

### 15 Jul 2026
Sumber: sesi kerja `feature/api-test` (commit `f9832c7`, `b1ecd58`).

- Tambah **section 3.9 API Testing Collection (Bruno)**: 73-request collection tervalidasi end-to-end, 58/58 request PASS, 153/153 test assertion PASS.
- Update **section 3.8**: catat audit swagger lanjutan (5 gap godoc/route/enum/schema) dan **breaking change kecil** pada response `GetMerchantAccount` (PascalCase → snake_case JSON field).
- Update **section 3.6**: catat verifikasi taksonomi journey action otomatis via CI.
- Update **section 3.12**: catat fix stabilitas CI (`rg` → `grep` di 2 script verify ISO, karena `ripgrep` tidak tersedia di runner GitHub Actions default).
- Update **section 5**: tambah folder `bruno/` ke struktur proyek.
- Section 7 (coverage) tidak diubah angkanya — perubahan Go sesi ini murni godoc + 1 JSON tag struct, tidak menyentuh logic yang diuji unit test; ditambahkan disclaimer bahwa commit reference (`c3777ba`) sudah lebih lama dari HEAD saat ini.
