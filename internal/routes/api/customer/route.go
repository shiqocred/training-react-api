package customer

import (
	"react-api/internal/middleware"
	"react-api/internal/utils"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct{ Pool *pgxpool.Pool }

func RegisterRoutes(router fiber.Router, pool *pgxpool.Pool) {
	h := Handler{Pool: pool}
	router.Get("/mutations", h.Mutations)
	router.Get("/mutations/:id", h.MutationDetail)
	router.Post("/deposit", h.Deposit)
	router.Post("/withdraw", h.Withdraw)
	router.Post("/transfer", h.Transfer)
}

type mutationItem struct {
	ID            string  `json:"id"`
	Type          string  `json:"type"`
	Direction     string  `json:"direction"`
	Amount        int64   `json:"amount"`
	BalanceBefore int64   `json:"balance_before"`
	BalanceAfter  int64   `json:"balance_after"`
	ReferenceID   *string `json:"reference_id"`
	Note          *string `json:"note"`
	ActorName     string  `json:"actor_name"`
	ActorRole     string  `json:"actor_role"`
	CreatedAt     string  `json:"created_at"`
}

func (h Handler) Mutations(c fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	page, perPage, offset := utils.GetPagination(c)
	keyword := "%" + strings.ToLower(c.Query("q", "")) + "%"
	typeQuery := strings.ToLower(strings.TrimSpace(c.Query("type", "")))
	direction := strings.ToLower(strings.TrimSpace(c.Query("direction", "")))
	actorRole := strings.ToLower(strings.TrimSpace(c.Query("actor_role", "")))
	dateFrom := strings.TrimSpace(c.Query("date_from", ""))
	dateTo := strings.TrimSpace(c.Query("date_to", ""))
	where := `t.customer_id=$1 AND ($2='' OR lower(t.type) LIKE $3 OR lower(t.direction) LIKE $3 OR lower(coalesce(t.note,'')) LIKE $3 OR lower(a.name) LIKE $3 OR lower(a.role) LIKE $3) AND ($4='' OR t.type=$4) AND ($5='' OR t.direction=$5) AND ($6='' OR a.role=$6) AND ($7='' OR t.created_at >= $7::timestamptz) AND ($8='' OR t.created_at < ($8::date + INTERVAL '1 day'))`
	args := []any{user.UserID, strings.Trim(c.Query("q", ""), " "), keyword, typeQuery, direction, actorRole, dateFrom, dateTo}
	var total int64
	if err := h.Pool.QueryRow(c.Context(), `SELECT count(*) FROM transactions t JOIN users a ON a.id=t.actor_id WHERE `+where, args...).Scan(&total); err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal menghitung mutasi")
	}
	args = append(args, perPage, offset)
	rows, err := h.Pool.Query(c.Context(), `SELECT t.id,t.type,t.direction,t.amount,t.balance_before,t.balance_after,t.reference_id,t.note,a.name,a.role,to_char(t.created_at,'YYYY-MM-DD HH24:MI:SS') FROM transactions t JOIN users a ON a.id=t.actor_id WHERE `+where+` ORDER BY t.created_at DESC LIMIT $9 OFFSET $10`, args...)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal mengambil mutasi")
	}
	defer rows.Close()
	items := []mutationItem{}
	for rows.Next() {
		var item mutationItem
		if err := rows.Scan(&item.ID, &item.Type, &item.Direction, &item.Amount, &item.BalanceBefore, &item.BalanceAfter, &item.ReferenceID, &item.Note, &item.ActorName, &item.ActorRole, &item.CreatedAt); err != nil {
			return utils.Error(c, fiber.StatusInternalServerError, "Gagal membaca mutasi")
		}
		items = append(items, item)
	}
	return utils.Success(c, fiber.StatusOK, "Mutasi berhasil diambil", utils.PaginatedData{Items: items, Pagination: utils.NewPagination(page, perPage, total)})
}

func (h Handler) MutationDetail(c fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	var item mutationItem
	err := h.Pool.QueryRow(c.Context(), `SELECT t.id,t.type,t.direction,t.amount,t.balance_before,t.balance_after,t.reference_id,t.note,a.name,a.role,to_char(t.created_at,'YYYY-MM-DD HH24:MI:SS') FROM transactions t JOIN users a ON a.id=t.actor_id WHERE t.id=$1 AND t.customer_id=$2`, c.Params("id"), user.UserID).Scan(&item.ID, &item.Type, &item.Direction, &item.Amount, &item.BalanceBefore, &item.BalanceAfter, &item.ReferenceID, &item.Note, &item.ActorName, &item.ActorRole, &item.CreatedAt)
	if err != nil {
		return utils.Error(c, fiber.StatusNotFound, "Mutasi tidak ditemukan")
	}
	return utils.Success(c, fiber.StatusOK, "Detail mutasi berhasil diambil", item)
}

type amountRequest struct {
	Amount string `json:"amount"`
	Note   string `json:"note"`
}
type transferRequest struct {
	ToCustomerID string `json:"to_customer_id"`
	Amount       string `json:"amount"`
	Note         string `json:"note"`
}

func (h Handler) Deposit(c fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	var req amountRequest
	if err := c.Bind().Body(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "Request tidak valid")
	}
	amount, err := utils.ParseAmountToCents(req.Amount)
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, err.Error())
	}
	tx, err := h.Pool.Begin(c.Context())
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal memulai transaksi database")
	}
	defer tx.Rollback(c.Context())
	result, err := Deposit(c.Context(), tx, user.UserID, user.UserID, amount, req.Note)
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, err.Error())
	}
	if err := tx.Commit(c.Context()); err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal menyimpan transaksi")
	}
	return utils.Success(c, fiber.StatusCreated, "Setor tunai berhasil", result)
}

func (h Handler) Withdraw(c fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	var req amountRequest
	if err := c.Bind().Body(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "Request tidak valid")
	}
	amount, err := utils.ParseAmountToCents(req.Amount)
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, err.Error())
	}
	tx, err := h.Pool.Begin(c.Context())
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal memulai transaksi database")
	}
	defer tx.Rollback(c.Context())
	result, err := Withdraw(c.Context(), tx, user.UserID, user.UserID, amount, req.Note)
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, err.Error())
	}
	if err := tx.Commit(c.Context()); err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal menyimpan transaksi")
	}
	return utils.Success(c, fiber.StatusCreated, "Tarik tunai berhasil", result)
}

func (h Handler) Transfer(c fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	var req transferRequest
	if err := c.Bind().Body(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "Request tidak valid")
	}
	amount, err := utils.ParseAmountToCents(req.Amount)
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, err.Error())
	}
	tx, err := h.Pool.Begin(c.Context())
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal memulai transaksi database")
	}
	defer tx.Rollback(c.Context())
	result, err := Transfer(c.Context(), tx, user.UserID, user.UserID, req.ToCustomerID, amount, req.Note)
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, err.Error())
	}
	if err := tx.Commit(c.Context()); err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal menyimpan transaksi")
	}
	return utils.Success(c, fiber.StatusCreated, "Transfer berhasil", result)
}
