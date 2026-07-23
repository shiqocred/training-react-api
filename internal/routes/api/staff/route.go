package staff

import (
	"strings"

	"react-api/internal/middleware"
	banking "react-api/internal/routes/api/customer"
	"react-api/internal/utils"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct{ Pool *pgxpool.Pool }

func RegisterRoutes(router fiber.Router, pool *pgxpool.Pool) {
	h := Handler{Pool: pool}
	router.Get("/customers", h.Customers)
	router.Get("/mutations", h.AllMutations)
	router.Post("/customers/:customer_id/deposit", h.Deposit)
	router.Post("/customers/:customer_id/withdraw", h.Withdraw)
	router.Post("/customers/:customer_id/transfer", h.Transfer)
}

type customerItem struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	AccountNumber string `json:"account_number"`
	Balance       int64  `json:"balance"`
	CreatedAt     string `json:"created_at"`
}

func (h Handler) Customers(c fiber.Ctx) error {
	page, perPage, offset := utils.GetPagination(c)
	keyword := "%" + strings.ToLower(c.Query("q", "")) + "%"
	var total int64
	if err := h.Pool.QueryRow(c.Context(), `SELECT count(*) FROM users WHERE role='customer' AND deleted_at IS NULL AND (lower(name) LIKE $1 OR lower(email) LIKE $1)`, keyword).Scan(&total); err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal menghitung customer")
	}
	rows, err := h.Pool.Query(c.Context(), `SELECT u.id,u.name,u.email,a.account_number,a.balance,to_char(u.created_at,'YYYY-MM-DD HH24:MI:SS') FROM users u JOIN accounts a ON a.user_id=u.id WHERE u.role='customer' AND u.deleted_at IS NULL AND (lower(u.name) LIKE $1 OR lower(u.email) LIKE $1) ORDER BY u.created_at DESC LIMIT $2 OFFSET $3`, keyword, perPage, offset)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal mengambil customer")
	}
	defer rows.Close()
	items := []customerItem{}
	for rows.Next() {
		var item customerItem
		if err := rows.Scan(&item.ID, &item.Name, &item.Email, &item.AccountNumber, &item.Balance, &item.CreatedAt); err != nil {
			return utils.Error(c, fiber.StatusInternalServerError, "Gagal membaca customer")
		}
		items = append(items, item)
	}
	return utils.Success(c, fiber.StatusOK, "Customer berhasil diambil", utils.PaginatedData{Items: items, Pagination: utils.NewPagination(page, perPage, total)})
}

type mutationItem struct {
	ID            string  `json:"id"`
	CustomerName  string  `json:"customer_name"`
	ActorName     string  `json:"actor_name"`
	ActorRole     string  `json:"actor_role"`
	Type          string  `json:"type"`
	Direction     string  `json:"direction"`
	Amount        int64   `json:"amount"`
	BalanceBefore int64   `json:"balance_before"`
	BalanceAfter  int64   `json:"balance_after"`
	ReferenceID   *string `json:"reference_id"`
	Note          *string `json:"note"`
	CreatedAt     string  `json:"created_at"`
}

func (h Handler) AllMutations(c fiber.Ctx) error {
	page, perPage, offset := utils.GetPagination(c)
	keyword := "%" + strings.ToLower(c.Query("q", "")) + "%"
	var total int64
	if err := h.Pool.QueryRow(c.Context(), `SELECT count(*) FROM transactions t JOIN users c ON c.id=t.customer_id JOIN users a ON a.id=t.actor_id WHERE lower(c.name) LIKE $1 OR lower(c.email) LIKE $1 OR lower(a.name) LIKE $1 OR lower(a.role) LIKE $1 OR lower(t.type) LIKE $1 OR lower(t.direction) LIKE $1 OR lower(coalesce(t.note,'')) LIKE $1`, keyword).Scan(&total); err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal menghitung mutasi")
	}
	rows, err := h.Pool.Query(c.Context(), `SELECT t.id,c.name,a.name,a.role,t.type,t.direction,t.amount,t.balance_before,t.balance_after,t.reference_id,t.note,to_char(t.created_at,'YYYY-MM-DD HH24:MI:SS') FROM transactions t JOIN users c ON c.id=t.customer_id JOIN users a ON a.id=t.actor_id WHERE lower(c.name) LIKE $1 OR lower(c.email) LIKE $1 OR lower(a.name) LIKE $1 OR lower(a.role) LIKE $1 OR lower(t.type) LIKE $1 OR lower(t.direction) LIKE $1 OR lower(coalesce(t.note,'')) LIKE $1 ORDER BY t.created_at DESC LIMIT $2 OFFSET $3`, keyword, perPage, offset)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal mengambil mutasi")
	}
	defer rows.Close()
	items := []mutationItem{}
	for rows.Next() {
		var item mutationItem
		if err := rows.Scan(&item.ID, &item.CustomerName, &item.ActorName, &item.ActorRole, &item.Type, &item.Direction, &item.Amount, &item.BalanceBefore, &item.BalanceAfter, &item.ReferenceID, &item.Note, &item.CreatedAt); err != nil {
			return utils.Error(c, fiber.StatusInternalServerError, "Gagal membaca mutasi")
		}
		items = append(items, item)
	}
	return utils.Success(c, fiber.StatusOK, "Mutasi semua customer berhasil diambil", utils.PaginatedData{Items: items, Pagination: utils.NewPagination(page, perPage, total)})
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

func (h Handler) Deposit(c fiber.Ctx) error  { return h.doAmount(c, "deposit") }
func (h Handler) Withdraw(c fiber.Ctx) error { return h.doAmount(c, "withdraw") }

func (h Handler) doAmount(c fiber.Ctx, op string) error {
	actor := middleware.CurrentUser(c)
	customerID := c.Params("customer_id")
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
	var result banking.OperationResult
	if op == "deposit" {
		result, err = banking.Deposit(c.Context(), tx, actor.UserID, customerID, amount, req.Note)
	} else {
		result, err = banking.Withdraw(c.Context(), tx, actor.UserID, customerID, amount, req.Note)
	}
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, err.Error())
	}
	if err := tx.Commit(c.Context()); err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal menyimpan transaksi")
	}
	msg := "Setor tunai customer berhasil"
	if op == "withdraw" {
		msg = "Tarik tunai customer berhasil"
	}
	return utils.Success(c, fiber.StatusCreated, msg, result)
}

func (h Handler) Transfer(c fiber.Ctx) error {
	actor := middleware.CurrentUser(c)
	fromID := c.Params("customer_id")
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
	result, err := banking.Transfer(c.Context(), tx, actor.UserID, fromID, req.ToCustomerID, amount, req.Note)
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, err.Error())
	}
	if err := tx.Commit(c.Context()); err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal menyimpan transaksi")
	}
	return utils.Success(c, fiber.StatusCreated, "Transfer customer berhasil", result)
}
