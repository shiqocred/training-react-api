package session

import (
	"react-api/internal/middleware"
	"react-api/internal/utils"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	Pool *pgxpool.Pool
}

func RegisterRoutes(router fiber.Router, pool *pgxpool.Pool) {
	h := Handler{Pool: pool}
	router.Get("/me", h.Me)
	router.Get("/check-auth", h.CheckAuth)
}

func (h Handler) Me(c fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	data := fiber.Map{
		"id":    user.UserID,
		"name":  user.Name,
		"email": user.Email,
		"role":  user.Role,
	}

	if user.Role == "customer" {
		var accountNumber string
		var balance int64
		err := h.Pool.QueryRow(c.Context(), `SELECT account_number, balance FROM accounts WHERE user_id=$1`, user.UserID).Scan(&accountNumber, &balance)
		if err != nil && err != pgx.ErrNoRows {
			return utils.Error(c, fiber.StatusInternalServerError, "Gagal mengambil informasi rekening")
		}
		if err == nil {
			data["account"] = fiber.Map{
				"account_number": accountNumber,
				"balance":        balance,
			}
		}
	}

	return utils.Success(c, fiber.StatusOK, "Data pengguna berhasil diambil", data)
}

func (h Handler) CheckAuth(c fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data":    nil,
		"message": "Session valid",
		"status":  true,
	})
}
