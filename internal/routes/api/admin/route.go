package admin

import (
	"strings"

	"react-api/internal/config"
	"react-api/internal/routes/api/auth"
	staffroute "react-api/internal/routes/api/staff"
	"react-api/internal/utils"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	Pool *pgxpool.Pool
	Cfg  config.Config
}

func RegisterRoutes(router fiber.Router, pool *pgxpool.Pool, cfg config.Config) {
	staffroute.RegisterRoutes(router, pool, cfg)
	h := Handler{Pool: pool, Cfg: cfg}
	router.Get("/staff", h.ListStaff)
	router.Post("/staff", h.CreateStaff)
	router.Put("/staff/:id", h.UpdateStaff)
	router.Delete("/staff/:id", h.DeleteStaff)
}

type staffItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
}

func (h Handler) ListStaff(c fiber.Ctx) error {
	page, perPage, offset := utils.GetPagination(c)
	keyword := "%" + strings.ToLower(c.Query("q", "")) + "%"
	var total int64
	if err := h.Pool.QueryRow(c.Context(), `SELECT count(*) FROM users WHERE role='staff' AND deleted_at IS NULL AND (lower(name) LIKE $1 OR lower(email) LIKE $1)`, keyword).Scan(&total); err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal menghitung staff")
	}
	rows, err := h.Pool.Query(c.Context(), `SELECT id,name,email,role,to_char(created_at,'YYYY-MM-DD HH24:MI:SS') FROM users WHERE role='staff' AND deleted_at IS NULL AND (lower(name) LIKE $1 OR lower(email) LIKE $1) ORDER BY created_at DESC LIMIT $2 OFFSET $3`, keyword, perPage, offset)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal mengambil staff")
	}
	defer rows.Close()
	items := []staffItem{}
	for rows.Next() {
		var item staffItem
		if err := rows.Scan(&item.ID, &item.Name, &item.Email, &item.Role, &item.CreatedAt); err != nil {
			return utils.Error(c, fiber.StatusInternalServerError, "Gagal membaca staff")
		}
		items = append(items, item)
	}
	return utils.Success(c, fiber.StatusOK, "Staff berhasil diambil", utils.PaginatedData{Items: items, Pagination: utils.NewPagination(page, perPage, total)})
}

type staffRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h Handler) CreateStaff(c fiber.Ctx) error {
	var req staffRequest
	if err := c.Bind().Body(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "Request tidak valid")
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Name == "" || !strings.Contains(req.Email, "@") || len(req.Password) < 8 {
		return utils.Error(c, fiber.StatusBadRequest, "Nama, email valid, dan password minimal 8 karakter wajib diisi")
	}
	hash, err := utils.HashPassword(req.Password, auth.ArgonParams(h.Cfg))
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal mengenkripsi password")
	}
	id, _ := utils.NewCUID2()
	_, err = h.Pool.Exec(c.Context(), `INSERT INTO users (id,name,email,password,role) VALUES ($1,$2,$3,$4,'staff')`, id, req.Name, req.Email, hash)
	if err != nil {
		return utils.Error(c, fiber.StatusConflict, "Email staff sudah digunakan")
	}
	return utils.Success(c, fiber.StatusCreated, "Staff berhasil dibuat", fiber.Map{"id": id, "name": req.Name, "email": req.Email, "role": "staff"})
}

func (h Handler) UpdateStaff(c fiber.Ctx) error {
	var req staffRequest
	if err := c.Bind().Body(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "Request tidak valid")
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Name == "" || !strings.Contains(req.Email, "@") {
		return utils.Error(c, fiber.StatusBadRequest, "Nama dan email valid wajib diisi")
	}
	if req.Password != "" {
		hash, err := utils.HashPassword(req.Password, auth.ArgonParams(h.Cfg))
		if err != nil {
			return utils.Error(c, fiber.StatusInternalServerError, "Gagal mengenkripsi password")
		}
		_, err = h.Pool.Exec(c.Context(), `UPDATE users SET name=$1,email=$2,password=$3,updated_at=now() WHERE id=$4 AND role='staff' AND deleted_at IS NULL`, req.Name, req.Email, hash, c.Params("id"))
		if err != nil {
			return utils.Error(c, fiber.StatusConflict, "Email staff sudah digunakan")
		}
	} else {
		_, err := h.Pool.Exec(c.Context(), `UPDATE users SET name=$1,email=$2,updated_at=now() WHERE id=$3 AND role='staff' AND deleted_at IS NULL`, req.Name, req.Email, c.Params("id"))
		if err != nil {
			return utils.Error(c, fiber.StatusConflict, "Email staff sudah digunakan")
		}
	}
	return utils.Success(c, fiber.StatusOK, "Staff berhasil diperbarui", fiber.Map{"id": c.Params("id"), "name": req.Name, "email": req.Email, "role": "staff"})
}

func (h Handler) DeleteStaff(c fiber.Ctx) error {
	_, err := h.Pool.Exec(c.Context(), `UPDATE users SET deleted_at=now(),updated_at=now() WHERE id=$1 AND role='staff' AND deleted_at IS NULL`, c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal menghapus staff")
	}
	return utils.Success(c, fiber.StatusOK, "Staff berhasil dihapus", nil)
}
