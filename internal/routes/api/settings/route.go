package settings

import (
	"strings"

	"react-api/internal/config"
	"react-api/internal/middleware"
	"react-api/internal/routes/api/auth"
	"react-api/internal/utils"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	Pool *pgxpool.Pool
	Cfg  config.Config
}

func RegisterRoutes(router fiber.Router, pool *pgxpool.Pool, cfg config.Config) {
	h := Handler{Pool: pool, Cfg: cfg}
	router.Put("/profile", h.UpdateProfile)
	router.Put("/password", h.UpdatePassword)
}

type profileRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (h Handler) UpdateProfile(c fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	var req profileRequest
	if err := c.Bind().Body(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "Request tidak valid")
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Name == "" {
		return utils.Error(c, fiber.StatusBadRequest, "Nama wajib diisi")
	}
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		return utils.Error(c, fiber.StatusBadRequest, "Email tidak valid")
	}
	_, err := h.Pool.Exec(c.Context(), `UPDATE users SET name=$1,email=$2,updated_at=now() WHERE id=$3`, req.Name, req.Email, user.UserID)
	if err != nil {
		return utils.Error(c, fiber.StatusConflict, "Email sudah digunakan")
	}
	return utils.Success(c, fiber.StatusOK, "Profil berhasil diperbarui", fiber.Map{"id": user.UserID, "name": req.Name, "email": req.Email, "role": user.Role})
}

type passwordRequest struct {
	OldPassword    string `json:"old_password"`
	NewPassword    string `json:"new_password"`
	VerifyPassword string `json:"verify_password"`
}

func (h Handler) UpdatePassword(c fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	var req passwordRequest
	if err := c.Bind().Body(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "Request tidak valid")
	}
	if req.NewPassword != req.VerifyPassword {
		return utils.Error(c, fiber.StatusBadRequest, "Konfirmasi password tidak sama")
	}
	if len(req.NewPassword) < 8 {
		return utils.Error(c, fiber.StatusBadRequest, "Password baru minimal 8 karakter")
	}
	var currentHash string
	if err := h.Pool.QueryRow(c.Context(), `SELECT password FROM users WHERE id=$1`, user.UserID).Scan(&currentHash); err != nil {
		return utils.Error(c, fiber.StatusNotFound, "Pengguna tidak ditemukan")
	}
	if !utils.VerifyPassword(req.OldPassword, currentHash) {
		return utils.Error(c, fiber.StatusBadRequest, "Password lama salah")
	}
	newHash, err := utils.HashPassword(req.NewPassword, auth.ArgonParams(h.Cfg))
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal mengenkripsi password")
	}
	_, err = h.Pool.Exec(c.Context(), `UPDATE users SET password=$1,updated_at=now() WHERE id=$2`, newHash, user.UserID)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal memperbarui password")
	}
	return utils.Success(c, fiber.StatusOK, "Password berhasil diperbarui", nil)
}
