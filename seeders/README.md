# Seeder

Seeder utama dibuat sebagai command Go agar password dapat di-hash dengan Argon2 dan ID memakai CUID2.

Jalankan:

```bash
go run ./cmd/seed
```

## Mode

Seeder membaca mode dari `SEED_MODE`. Jika kosong, akan membaca `NODE_ENV`.

- `NODE_ENV=production` atau `SEED_MODE=production`: hanya membuat/update admin dari env.
- Non-production: membuat data demo dan transaksi demo. Transaksi demo hanya jalan sekali memakai tabel `seed_runs`.

## Production

Wajib mengisi:

```env
SEED_ADMIN_NAME=Admin
SEED_ADMIN_EMAIL=admin@example.com
SEED_ADMIN_PASSWORD=password-kuat-minimal-12
```

Contoh:

```bash
NODE_ENV=production go run ./cmd/seed
```

## Development

Akun awal:

- `admin@example.com` / `password123`
- `staff@example.com` / `password123`
- `customer@example.com` / `password123`
- `customer2@example.com` / `password123`
