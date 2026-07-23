package middleware

import (
	"strings"

	"react-api/internal/config"
	"react-api/internal/utils"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthContext struct {
	UserID string
	Role   string
	Name   string
	Email  string
}

func Auth(pool *pgxpool.Pool, cfg config.Config) fiber.Handler {
	return func(c fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			return utils.Error(c, fiber.StatusUnauthorized, "Token bearer wajib dikirim")
		}
		key, err := utils.EnsureBearerKey(c.Context(), pool)
		if err != nil {
			return utils.Error(c, fiber.StatusInternalServerError, "Gagal mengambil key autentikasi")
		}
		claims, err := utils.ParseBearerToken(strings.TrimSpace(authHeader[7:]), cfg.AuthIssuer, key.PublicKey)
		if err != nil {
			return utils.Error(c, fiber.StatusUnauthorized, "Token tidak valid atau sudah kedaluwarsa")
		}

		var ctx AuthContext
		err = pool.QueryRow(c.Context(), `SELECT id, name, email, role FROM users WHERE id=$1 AND deleted_at IS NULL`, claims.UserID).Scan(&ctx.UserID, &ctx.Name, &ctx.Email, &ctx.Role)
		if err != nil {
			return utils.Error(c, fiber.StatusUnauthorized, "Pengguna tidak ditemukan")
		}
		c.Locals("auth", ctx)
		return c.Next()
	}
}

func Role(roles ...string) fiber.Handler {
	allowed := map[string]bool{}
	for _, role := range roles {
		allowed[role] = true
	}
	return func(c fiber.Ctx) error {
		ctx, ok := c.Locals("auth").(AuthContext)
		if !ok {
			return utils.Error(c, fiber.StatusUnauthorized, "Anda belum login")
		}
		if !allowed[ctx.Role] {
			return utils.Error(c, fiber.StatusForbidden, "Anda tidak memiliki akses")
		}
		return c.Next()
	}
}

func CurrentUser(c fiber.Ctx) AuthContext {
	ctx, _ := c.Locals("auth").(AuthContext)
	return ctx
}
