# Fiber Banking API

API perbankan sederhana dengan Go Fiber, PostgreSQL, bearer auth, Argon2 password hashing, CUID2 IDs via `github.com/nrednav/cuid2`, migrasi, dan seeder.

## Dokumentasi OpenAPI

Spesifikasi lengkap tersedia di `docs/openapi.yaml`.

## Menjalankan

```bash
cp .env.example .env
# edit DATABASE_URL dan RESEND_API_KEY jika ingin OTP dikirim email sungguhan
go mod tidy
go run ./cmd/api
```

Migrasi otomatis dijalankan saat API start.

Seeder:

```bash
go run ./cmd/seed
```

Mode seeder:

- `NODE_ENV=production` atau `SEED_MODE=production`: hanya membuat/update admin dari `SEED_ADMIN_NAME`, `SEED_ADMIN_EMAIL`, dan `SEED_ADMIN_PASSWORD`.
- Non-production: membuat admin, staff, customer demo, dan transaksi demo. Transaksi demo hanya dijalankan sekali memakai tabel `seed_runs`.

Production seed wajib memakai password kuat minimal 12 karakter:

```bash
NODE_ENV=production \
SEED_ADMIN_NAME="Admin" \
SEED_ADMIN_EMAIL="admin@example.com" \
SEED_ADMIN_PASSWORD="password-kuat-minimal-12" \
go run ./cmd/seed
```

## Docker

Build image:

```bash
docker build -t fiber-banking-api .
```

Run API langsung:

```bash
docker run --rm -p 3000:3000 --env-file .env fiber-banking-api
```

Contoh production seed dengan `docker run`:

```bash
docker run --rm --env-file .env \
  -e NODE_ENV=production \
  -e SEED_ADMIN_NAME="Admin" \
  -e SEED_ADMIN_EMAIL="admin@example.com" \
  -e SEED_ADMIN_PASSWORD="password-kuat-minimal-12" \
  fiber-banking-api seed
```

### Docker Compose di server

File compose sengaja tidak disimpan di repository API agar server bisa menggabungkan beberapa repo/service, misalnya API, PostgreSQL, Traefik, dan nanti frontend dari repo lain.

Simpan compose di server, misalnya:

```txt
/opt/banking/docker-compose.yml
/opt/banking/.env
/opt/banking/api/      # clone repo ini
/opt/banking/frontend/ # nanti clone repo frontend
```

Traefik expose port `80` dan `443`, lalu meneruskan request domain API ke container API port internal `3000`. Seeder sebaiknya dijalankan sebagai one-off job/container terpisah, bukan sebagai service yang selalu hidup.

Jika memakai service `seed` di compose server, pasang profile agar tidak ikut jalan saat deploy biasa:

```yaml
seed:
  image: banking-api:latest
  profiles:
    - tools
  command: ["seed"]
```

Dengan profile tersebut, seeder **tidak akan jalan** saat rebuild/redeploy biasa:

```bash
docker compose up -d --build
```

Seeder hanya jalan kalau dipanggil manual:

```bash
docker compose --profile tools run --rm seed
```

Email OTP dikirim menggunakan SDK resmi `github.com/resend/resend-go/v3`. Jika `RESEND_API_KEY` kosong, OTP akan ditulis ke stdout untuk development.

## Response standar

Sukses:

```json
{ "data": {}, "status": true, "message": "Pesan bahasa Indonesia" }
```

Error:

```json
{ "status": false, "message": "Pesan error bahasa Indonesia" }
```

Pagination list:

```json
{
  "data": {
    "items": [],
    "pagination": {
      "current_page": 1,
      "per_page": 10,
      "from": 1,
      "total": 20,
      "last_page": 2
    }
  },
  "status": true,
  "message": "..."
}
```

## Endpoint utama

Auth:

- `POST /api/auth/register` customer-only
- `POST /api/auth/login`
- `POST /api/auth/forgot-password`
- `POST /api/auth/verify-otp`

Session (Bearer):

- `GET /api/me`
- `GET /api/check-auth`

Settings (Bearer):

- `PUT /api/settings/profile`
- `PUT /api/settings/password`

Customer (role customer):

- `GET /api/customer/mutations?page=1&per_page=10`
- `POST /api/customer/deposit`
- `POST /api/customer/withdraw`
- `POST /api/customer/transfer`

Staff/Admin teller:

- `GET /api/staff/customers`
- `GET /api/staff/mutations`
- `POST /api/staff/customers/:customer_id/deposit`
- `POST /api/staff/customers/:customer_id/withdraw`
- `POST /api/staff/customers/:customer_id/transfer`

Admin:

- `GET /api/admin/staff`
- `POST /api/admin/staff`
- `PUT /api/admin/staff/:id`
- `DELETE /api/admin/staff/:id`
- Admin juga bisa mengakses fitur teller pada prefix `/api/admin/...`.
