package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"react-api/internal/config"
	"react-api/internal/database"
	"react-api/internal/routes/api/auth"
	banking "react-api/internal/routes/api/customer"
	"react-api/internal/utils"

	"github.com/joho/godotenv"
)

const developmentSeedName = "development_seed_v1"

func main() {
	_ = godotenv.Load()
	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx, "migrations"); err != nil {
		log.Fatal(err)
	}

	mode := seedMode()
	if mode == "production" {
		seedProduction(ctx, db, cfg)
		return
	}
	seedDevelopment(ctx, db, cfg)
}

func seedMode() string {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("SEED_MODE")))
	if mode != "" {
		return mode
	}
	nodeEnv := strings.ToLower(strings.TrimSpace(os.Getenv("NODE_ENV")))
	if nodeEnv == "production" {
		return "production"
	}
	return "development"
}

func seedProduction(ctx context.Context, db *database.DB, cfg config.Config) {
	name := strings.TrimSpace(os.Getenv("SEED_ADMIN_NAME"))
	email := strings.ToLower(strings.TrimSpace(os.Getenv("SEED_ADMIN_EMAIL")))
	password := os.Getenv("SEED_ADMIN_PASSWORD")

	if name == "" || email == "" || password == "" {
		log.Fatal("SEED_ADMIN_NAME, SEED_ADMIN_EMAIL, dan SEED_ADMIN_PASSWORD wajib diisi untuk production")
	}
	if len(password) < 12 {
		log.Fatal("SEED_ADMIN_PASSWORD untuk production minimal 12 karakter")
	}

	adminID := upsertUser(ctx, db, cfg, email, name, "admin", password)
	log.Printf("Seeder production selesai. Admin tersedia: %s (%s)", email, adminID)
}

func seedDevelopment(ctx context.Context, db *database.DB, cfg config.Config) {
	adminID := upsertUser(ctx, db, cfg, "admin@example.com", "Admin Utama", "admin", "password123")
	staffID := upsertUser(ctx, db, cfg, "staff@example.com", "Staff Teller", "staff", "password123")
	customer1ID := upsertUser(ctx, db, cfg, "customer@example.com", "Customer Satu", "customer", "password123")
	customer2ID := upsertUser(ctx, db, cfg, "customer2@example.com", "Customer Dua", "customer", "password123")
	ensureAccount(ctx, db, customer1ID, "1000000001")
	ensureAccount(ctx, db, customer2ID, "1000000002")

	alreadyRan := seedAlreadyRan(ctx, db, developmentSeedName)
	if alreadyRan {
		log.Println("Seeder development sudah pernah dijalankan, transaksi demo dilewati")
		log.Println("Login: admin@example.com / staff@example.com / customer@example.com / customer2@example.com dengan password password123")
		return
	}

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := banking.Deposit(ctx, tx, adminID, customer1ID, 100000000, "Saldo awal customer satu"); err != nil {
		log.Fatal(err)
	}
	if _, err := banking.Deposit(ctx, tx, staffID, customer2ID, 50000000, "Saldo awal customer dua"); err != nil {
		log.Fatal(err)
	}
	if _, err := banking.Transfer(ctx, tx, customer1ID, customer1ID, customer2ID, 1250000, "Transfer awal antar customer"); err != nil {
		log.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO seed_runs (name) VALUES ($1) ON CONFLICT (name) DO NOTHING`, developmentSeedName); err != nil {
		log.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		log.Fatal(err)
	}
	log.Println("Seeder development berhasil dijalankan. Login: admin@example.com / staff@example.com / customer@example.com / customer2@example.com dengan password password123")
}

func seedAlreadyRan(ctx context.Context, db *database.DB, name string) bool {
	var exists bool
	if err := db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM seed_runs WHERE name=$1)`, name).Scan(&exists); err != nil {
		log.Fatal(err)
	}
	return exists
}

func upsertUser(ctx context.Context, db *database.DB, cfg config.Config, email, name, role, password string) string {
	hash, err := utils.HashPassword(password, auth.ArgonParams(cfg))
	if err != nil {
		log.Fatal(err)
	}
	id, err := utils.NewCUID2()
	if err != nil {
		log.Fatal(err)
	}
	_, err = db.Pool.Exec(ctx, `INSERT INTO users (id,name,email,password,role) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (email) DO UPDATE SET name=EXCLUDED.name,password=EXCLUDED.password,role=EXCLUDED.role,deleted_at=NULL,updated_at=now()`, id, name, email, hash, role)
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `SELECT id FROM users WHERE email=$1`, email).Scan(&id); err != nil {
		log.Fatal(err)
	}
	return id
}

func ensureAccount(ctx context.Context, db *database.DB, userID, accountNumber string) {
	var exists bool
	if err := db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM accounts WHERE user_id=$1)`, userID).Scan(&exists); err != nil {
		log.Fatal(err)
	}
	if exists {
		return
	}
	id, err := utils.NewCUID2()
	if err != nil {
		log.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `INSERT INTO accounts (id,user_id,account_number,balance) VALUES ($1,$2,$3,0)`, id, userID, accountNumber); err != nil {
		log.Fatal(err)
	}
}
