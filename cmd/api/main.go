package main

import (
	"context"
	"log"
	"time"

	"react-api/internal/config"
	"react-api/internal/database"
	"react-api/internal/middleware"
	"react-api/internal/routes/api/admin"
	"react-api/internal/routes/api/auth"
	"react-api/internal/routes/api/customer"
	"react-api/internal/routes/api/session"
	"react-api/internal/routes/api/settings"
	"react-api/internal/routes/api/staff"
	"react-api/internal/utils"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx, "migrations"); err != nil {
		log.Fatal(err)
	}
	if _, err := utils.EnsureBearerKey(ctx, db.Pool); err != nil {
		log.Fatal(err)
	}

	app := fiber.New(fiber.Config{AppName: cfg.AppName})
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{AllowOrigins: []string{"*"}, AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"}, AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}}))
	app.Use(middleware.EndpointLogger(db.Pool))
	app.Get("/health", func(c fiber.Ctx) error {
		return utils.Success(c, fiber.StatusOK, "API berjalan", fiber.Map{"app": cfg.AppName})
	})

	api := app.Group("/api")
	auth.RegisterRoutes(api.Group("/auth"), db.Pool, cfg)

	secured := api.Group("", middleware.Auth(db.Pool, cfg))
	session.RegisterRoutes(secured, db.Pool)
	settings.RegisterRoutes(secured.Group("/settings"), db.Pool, cfg)
	customer.RegisterRoutes(secured.Group("/customer", middleware.Role("customer")), db.Pool)
	staff.RegisterRoutes(secured.Group("/staff", middleware.Role("staff", "admin")), db.Pool)
	admin.RegisterRoutes(secured.Group("/admin", middleware.Role("admin")), db.Pool, cfg)

	log.Fatal(app.Listen(":" + cfg.AppPort))
}
