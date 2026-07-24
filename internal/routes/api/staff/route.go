package staff

import (
	"strings"
	"time"

	"react-api/internal/config"
	"react-api/internal/middleware"
	"react-api/internal/routes/api/auth"
	banking "react-api/internal/routes/api/customer"
	"react-api/internal/utils"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	Pool *pgxpool.Pool
	Cfg  config.Config
}

func RegisterRoutes(router fiber.Router, pool *pgxpool.Pool, cfg config.Config) {
	h := Handler{Pool: pool, Cfg: cfg}
	router.Get("/dashboard", h.Dashboard)
	router.Get("/customers", h.Customers)
	router.Get("/customers/options", h.CustomerOptions)
	router.Post("/customers", h.CreateCustomer)
	router.Get("/customers/:customer_id", h.CustomerDetail)
	router.Put("/customers/:customer_id", h.UpdateCustomer)
	router.Delete("/customers/:customer_id", h.DeleteCustomer)
	router.Get("/mutations", h.AllMutations)
	router.Get("/mutations/:id", h.MutationDetail)
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
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type customerDetailData struct {
	customerItem
	Mutations []mutationItem `json:"mutations"`
}

type customerOptionItem struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	AccountNumber string `json:"account_number"`
}

func (h Handler) Customers(c fiber.Ctx) error {
	page, perPage, offset := utils.GetPagination(c)
	keyword := "%" + strings.ToLower(c.Query("q", "")) + "%"
	status := strings.ToLower(strings.TrimSpace(c.Query("status", "")))

	var total int64
	if err := h.Pool.QueryRow(c.Context(), `SELECT count(*) FROM users u JOIN accounts a ON a.user_id=u.id WHERE u.role='customer' AND u.deleted_at IS NULL AND ($2='' OR u.status=$2) AND (lower(u.name) LIKE $1 OR lower(u.email) LIKE $1 OR lower(a.account_number) LIKE $1)`, keyword, status).Scan(&total); err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal menghitung customer")
	}
	rows, err := h.Pool.Query(c.Context(), `SELECT u.id,u.name,u.email,a.account_number,a.balance,u.status,to_char(u.created_at,'YYYY-MM-DD HH24:MI:SS'),to_char(u.updated_at,'YYYY-MM-DD HH24:MI:SS') FROM users u JOIN accounts a ON a.user_id=u.id WHERE u.role='customer' AND u.deleted_at IS NULL AND ($2='' OR u.status=$2) AND (lower(u.name) LIKE $1 OR lower(u.email) LIKE $1 OR lower(a.account_number) LIKE $1) ORDER BY u.created_at DESC LIMIT $3 OFFSET $4`, keyword, status, perPage, offset)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal mengambil customer")
	}
	defer rows.Close()
	items := []customerItem{}
	for rows.Next() {
		var item customerItem
		if err := rows.Scan(&item.ID, &item.Name, &item.Email, &item.AccountNumber, &item.Balance, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return utils.Error(c, fiber.StatusInternalServerError, "Gagal membaca customer")
		}
		items = append(items, item)
	}
	return utils.Success(c, fiber.StatusOK, "Customer berhasil diambil", utils.PaginatedData{Items: items, Pagination: utils.NewPagination(page, perPage, total)})
}

func (h Handler) CustomerOptions(c fiber.Ctx) error {
	rows, err := h.Pool.Query(c.Context(), `SELECT u.id,u.name,u.email,a.account_number FROM users u JOIN accounts a ON a.user_id=u.id WHERE u.role='customer' AND u.status='active' AND u.deleted_at IS NULL ORDER BY u.name ASC`)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal mengambil opsi customer")
	}
	defer rows.Close()

	items := []customerOptionItem{}
	for rows.Next() {
		var item customerOptionItem
		if err := rows.Scan(&item.ID, &item.Name, &item.Email, &item.AccountNumber); err != nil {
			return utils.Error(c, fiber.StatusInternalServerError, "Gagal membaca opsi customer")
		}
		items = append(items, item)
	}

	return utils.Success(c, fiber.StatusOK, "Opsi customer berhasil diambil", items)
}

type customerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Status   string `json:"status"`
}

func (h Handler) CreateCustomer(c fiber.Ctx) error {
	var req customerRequest
	if err := c.Bind().Body(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "Request tidak valid")
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Status = normalizeCustomerStatus(req.Status)
	if req.Name == "" || !strings.Contains(req.Email, "@") || len(req.Password) < 8 {
		return utils.Error(c, fiber.StatusBadRequest, "Nama, email valid, dan password minimal 8 karakter wajib diisi")
	}
	hash, err := utils.HashPassword(req.Password, auth.ArgonParams(h.Cfg))
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal mengenkripsi password")
	}
	userID, _ := utils.NewCUID2()
	accountID, _ := utils.NewCUID2()
	accountNumber := time.Now().Format("20060102150405") + userID[len(userID)-6:]
	tx, err := h.Pool.Begin(c.Context())
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal memulai transaksi database")
	}
	defer tx.Rollback(c.Context())
	_, err = tx.Exec(c.Context(), `INSERT INTO users (id,name,email,password,role,status) VALUES ($1,$2,$3,$4,'customer',$5)`, userID, req.Name, req.Email, hash, req.Status)
	if err != nil {
		return utils.Error(c, fiber.StatusConflict, "Email customer sudah digunakan")
	}
	_, err = tx.Exec(c.Context(), `INSERT INTO accounts (id,user_id,account_number,balance) VALUES ($1,$2,$3,0)`, accountID, userID, accountNumber)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal membuat rekening customer")
	}
	if err := tx.Commit(c.Context()); err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal menyimpan customer")
	}
	return utils.Success(c, fiber.StatusCreated, "Customer berhasil dibuat", fiber.Map{"id": userID, "name": req.Name, "email": req.Email, "role": "customer", "account_number": accountNumber, "balance": 0, "status": req.Status})
}

func (h Handler) CustomerDetail(c fiber.Ctx) error {
	customerID := c.Params("customer_id")
	var data customerDetailData
	err := h.Pool.QueryRow(c.Context(), `SELECT u.id,u.name,u.email,a.account_number,a.balance,u.status,to_char(u.created_at,'YYYY-MM-DD HH24:MI:SS'),to_char(u.updated_at,'YYYY-MM-DD HH24:MI:SS') FROM users u JOIN accounts a ON a.user_id=u.id WHERE u.id=$1 AND u.role='customer' AND u.deleted_at IS NULL`, customerID).Scan(&data.ID, &data.Name, &data.Email, &data.AccountNumber, &data.Balance, &data.Status, &data.CreatedAt, &data.UpdatedAt)
	if err == pgx.ErrNoRows {
		return utils.Error(c, fiber.StatusNotFound, "Customer tidak ditemukan")
	}
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal mengambil detail customer")
	}
	mutations, err := h.listMutations(c, customerID, 10, 0)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal mengambil mutasi customer")
	}
	data.Mutations = mutations
	return utils.Success(c, fiber.StatusOK, "Detail customer berhasil diambil", data)
}

func (h Handler) UpdateCustomer(c fiber.Ctx) error {
	var req customerRequest
	if err := c.Bind().Body(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "Request tidak valid")
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Status = normalizeCustomerStatus(req.Status)
	if req.Name == "" || !strings.Contains(req.Email, "@") {
		return utils.Error(c, fiber.StatusBadRequest, "Nama dan email valid wajib diisi")
	}
	customerID := c.Params("customer_id")
	if req.Password != "" {
		if len(req.Password) < 8 {
			return utils.Error(c, fiber.StatusBadRequest, "Password minimal 8 karakter")
		}
		hash, err := utils.HashPassword(req.Password, auth.ArgonParams(h.Cfg))
		if err != nil {
			return utils.Error(c, fiber.StatusInternalServerError, "Gagal mengenkripsi password")
		}
		_, err = h.Pool.Exec(c.Context(), `UPDATE users SET name=$1,email=$2,password=$3,status=$4,updated_at=now() WHERE id=$5 AND role='customer' AND deleted_at IS NULL`, req.Name, req.Email, hash, req.Status, customerID)
		if err != nil {
			return utils.Error(c, fiber.StatusConflict, "Email customer sudah digunakan")
		}
	} else {
		_, err := h.Pool.Exec(c.Context(), `UPDATE users SET name=$1,email=$2,status=$3,updated_at=now() WHERE id=$4 AND role='customer' AND deleted_at IS NULL`, req.Name, req.Email, req.Status, customerID)
		if err != nil {
			return utils.Error(c, fiber.StatusConflict, "Email customer sudah digunakan")
		}
	}
	return utils.Success(c, fiber.StatusOK, "Customer berhasil diperbarui", fiber.Map{"id": customerID, "name": req.Name, "email": req.Email, "role": "customer", "status": req.Status})
}

func (h Handler) DeleteCustomer(c fiber.Ctx) error {
	_, err := h.Pool.Exec(c.Context(), `UPDATE users SET status='inactive',deleted_at=now(),updated_at=now() WHERE id=$1 AND role='customer' AND deleted_at IS NULL`, c.Params("customer_id"))
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal menghapus customer")
	}
	return utils.Success(c, fiber.StatusOK, "Customer berhasil dihapus", nil)
}

type mutationItem struct {
	ID            string  `json:"id"`
	CustomerID    string  `json:"customer_id,omitempty"`
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
	typeQuery := strings.ToLower(strings.TrimSpace(c.Query("type", "")))
	direction := strings.ToLower(strings.TrimSpace(c.Query("direction", "")))
	customerID := strings.TrimSpace(c.Query("customer_id", ""))
	actorRole := strings.ToLower(strings.TrimSpace(c.Query("actor_role", "")))
	dateFrom := strings.TrimSpace(c.Query("date_from", ""))
	dateTo := strings.TrimSpace(c.Query("date_to", ""))

	where := `($1='' OR lower(c.name) LIKE $2 OR lower(c.email) LIKE $2 OR lower(a.name) LIKE $2 OR lower(a.role) LIKE $2 OR lower(t.type) LIKE $2 OR lower(t.direction) LIKE $2 OR lower(coalesce(t.note,'')) LIKE $2) AND ($3='' OR t.type=$3) AND ($4='' OR t.direction=$4) AND ($5='' OR t.customer_id=$5) AND ($6='' OR a.role=$6) AND ($7='' OR t.created_at >= $7::timestamptz) AND ($8='' OR t.created_at < ($8::date + INTERVAL '1 day'))`
	args := []any{strings.Trim(c.Query("q", ""), " "), keyword, typeQuery, direction, customerID, actorRole, dateFrom, dateTo}

	var total int64
	if err := h.Pool.QueryRow(c.Context(), `SELECT count(*) FROM transactions t JOIN users c ON c.id=t.customer_id JOIN users a ON a.id=t.actor_id WHERE `+where, args...).Scan(&total); err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal menghitung mutasi")
	}
	args = append(args, perPage, offset)
	rows, err := h.Pool.Query(c.Context(), `SELECT t.id,t.customer_id,c.name,a.name,a.role,t.type,t.direction,t.amount,t.balance_before,t.balance_after,t.reference_id,t.note,to_char(t.created_at,'YYYY-MM-DD HH24:MI:SS') FROM transactions t JOIN users c ON c.id=t.customer_id JOIN users a ON a.id=t.actor_id WHERE `+where+` ORDER BY t.created_at DESC LIMIT $9 OFFSET $10`, args...)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal mengambil mutasi")
	}
	defer rows.Close()
	items := []mutationItem{}
	for rows.Next() {
		var item mutationItem
		if err := rows.Scan(&item.ID, &item.CustomerID, &item.CustomerName, &item.ActorName, &item.ActorRole, &item.Type, &item.Direction, &item.Amount, &item.BalanceBefore, &item.BalanceAfter, &item.ReferenceID, &item.Note, &item.CreatedAt); err != nil {
			return utils.Error(c, fiber.StatusInternalServerError, "Gagal membaca mutasi")
		}
		items = append(items, item)
	}
	return utils.Success(c, fiber.StatusOK, "Mutasi semua customer berhasil diambil", utils.PaginatedData{Items: items, Pagination: utils.NewPagination(page, perPage, total)})
}

func (h Handler) MutationDetail(c fiber.Ctx) error {
	var item mutationItem
	err := h.Pool.QueryRow(c.Context(), `SELECT t.id,t.customer_id,c.name,a.name,a.role,t.type,t.direction,t.amount,t.balance_before,t.balance_after,t.reference_id,t.note,to_char(t.created_at,'YYYY-MM-DD HH24:MI:SS') FROM transactions t JOIN users c ON c.id=t.customer_id JOIN users a ON a.id=t.actor_id WHERE t.id=$1`, c.Params("id")).Scan(&item.ID, &item.CustomerID, &item.CustomerName, &item.ActorName, &item.ActorRole, &item.Type, &item.Direction, &item.Amount, &item.BalanceBefore, &item.BalanceAfter, &item.ReferenceID, &item.Note, &item.CreatedAt)
	if err == pgx.ErrNoRows {
		return utils.Error(c, fiber.StatusNotFound, "Mutasi tidak ditemukan")
	}
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal mengambil detail mutasi")
	}
	return utils.Success(c, fiber.StatusOK, "Detail mutasi berhasil diambil", item)
}

type dashboardPoint struct {
	Date    string `json:"date"`
	Income  int64  `json:"income"`
	Outcome int64  `json:"outcome"`
}

type dashboardSummary struct {
	TotalCustomers int64            `json:"total_customers"`
	TotalBalance   int64            `json:"total_balance"`
	TotalIncome    int64            `json:"total_income"`
	TotalOutcome   int64            `json:"total_outcome"`
	Chart          []dashboardPoint `json:"chart"`
}

func (h Handler) Dashboard(c fiber.Ctx) error {
	var summary dashboardSummary
	if err := h.Pool.QueryRow(c.Context(), `SELECT count(*), coalesce(sum(a.balance),0) FROM users u JOIN accounts a ON a.user_id=u.id WHERE u.role='customer' AND u.deleted_at IS NULL`).Scan(&summary.TotalCustomers, &summary.TotalBalance); err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal mengambil ringkasan customer")
	}
	if err := h.Pool.QueryRow(c.Context(), `SELECT coalesce(sum(amount) FILTER (WHERE direction='in'),0), coalesce(sum(amount) FILTER (WHERE direction='out'),0) FROM transactions WHERE created_at >= now() - INTERVAL '30 days'`).Scan(&summary.TotalIncome, &summary.TotalOutcome); err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal mengambil ringkasan transaksi")
	}
	rows, err := h.Pool.Query(c.Context(), `SELECT to_char(day,'YYYY-MM-DD') AS date, coalesce(sum(t.amount) FILTER (WHERE t.direction='in'),0) AS income, coalesce(sum(t.amount) FILTER (WHERE t.direction='out'),0) AS outcome FROM generate_series(current_date - INTERVAL '29 days', current_date, INTERVAL '1 day') day LEFT JOIN transactions t ON t.created_at::date=day::date GROUP BY day ORDER BY day`)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal mengambil grafik transaksi")
	}
	defer rows.Close()
	summary.Chart = []dashboardPoint{}
	for rows.Next() {
		var point dashboardPoint
		if err := rows.Scan(&point.Date, &point.Income, &point.Outcome); err != nil {
			return utils.Error(c, fiber.StatusInternalServerError, "Gagal membaca grafik transaksi")
		}
		summary.Chart = append(summary.Chart, point)
	}
	return utils.Success(c, fiber.StatusOK, "Ringkasan dashboard berhasil diambil", summary)
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

func (h Handler) listMutations(c fiber.Ctx, customerID string, limit int, offset int) ([]mutationItem, error) {
	rows, err := h.Pool.Query(c.Context(), `SELECT t.id,t.customer_id,c.name,a.name,a.role,t.type,t.direction,t.amount,t.balance_before,t.balance_after,t.reference_id,t.note,to_char(t.created_at,'YYYY-MM-DD HH24:MI:SS') FROM transactions t JOIN users c ON c.id=t.customer_id JOIN users a ON a.id=t.actor_id WHERE t.customer_id=$1 ORDER BY t.created_at DESC LIMIT $2 OFFSET $3`, customerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []mutationItem{}
	for rows.Next() {
		var item mutationItem
		if err := rows.Scan(&item.ID, &item.CustomerID, &item.CustomerName, &item.ActorName, &item.ActorRole, &item.Type, &item.Direction, &item.Amount, &item.BalanceBefore, &item.BalanceAfter, &item.ReferenceID, &item.Note, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func normalizeCustomerStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "inactive" || status == "blocked" {
		return status
	}
	return "active"
}
