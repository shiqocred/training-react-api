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

Run API:

```bash
docker run --rm -p 3000:3000 --env-file .env fiber-banking-api
```

Run seeder dari image yang sama:

```bash
docker run --rm --env-file .env fiber-banking-api seed
```

Contoh production seed:

```bash
docker run --rm --env-file .env \
  -e NODE_ENV=production \
  -e SEED_ADMIN_NAME="Admin" \
  -e SEED_ADMIN_EMAIL="admin@example.com" \
  -e SEED_ADMIN_PASSWORD="password-kuat-minimal-12" \
  fiber-banking-api seed
```

### Traefik

Image ini kompatibel dengan Traefik karena API listen ke `APP_PORT` dan Dockerfile expose `3000`. Pastikan service Traefik mengarah ke port internal yang sama dengan `APP_PORT`.

Contoh label Docker Compose:

```yaml
services:
  api:
    image: fiber-banking-api
    env_file: .env
    environment:
      NODE_ENV: production
      APP_PORT: 3000
    labels:
      - traefik.enable=true
      - traefik.http.routers.banking-api.rule=Host(`api.example.com`)
      - traefik.http.routers.banking-api.entrypoints=websecure
      - traefik.http.routers.banking-api.tls.certresolver=letsencrypt
      - traefik.http.services.banking-api.loadbalancer.server.port=3000
```

Seeder sebaiknya dijalankan sebagai one-off job/container terpisah, bukan sebagai service yang selalu hidup.

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
