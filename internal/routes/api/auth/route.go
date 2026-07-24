package auth

import (
	"strings"
	"time"

	"react-api/internal/config"
	"react-api/internal/utils"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	Pool  *pgxpool.Pool
	Cfg   config.Config
	Email utils.EmailSender
}

func RegisterRoutes(router fiber.Router, pool *pgxpool.Pool, cfg config.Config) {
	h := Handler{Pool: pool, Cfg: cfg, Email: utils.NewEmailSender(cfg.ResendAPIKey, cfg.EmailFrom)}
	router.Post("/register", h.Register)
	router.Post("/login", h.Login)
	router.Post("/forgot-password", h.ForgotPassword)
	router.Post("/verify-otp", h.VerifyOTP)
	router.Post("/reset-password", h.ResetPassword)
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h Handler) Register(c fiber.Ctx) error {
	var req registerRequest
	if err := c.Bind().Body(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "Request tidak valid")
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if msg := validateNameEmailPassword(req.Name, req.Email, req.Password); msg != "" {
		return utils.Error(c, fiber.StatusBadRequest, msg)
	}
	passwordHash, err := utils.HashPassword(req.Password, ArgonParams(h.Cfg))
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
	_, err = tx.Exec(c.Context(), `INSERT INTO users (id, name, email, password, role) VALUES ($1,$2,$3,$4,'customer')`, userID, strings.TrimSpace(req.Name), req.Email, passwordHash)
	if err != nil {
		return utils.Error(c, fiber.StatusConflict, "Email sudah terdaftar")
	}
	_, err = tx.Exec(c.Context(), `INSERT INTO accounts (id, user_id, account_number, balance) VALUES ($1,$2,$3,0)`, accountID, userID, accountNumber)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal membuat rekening customer")
	}
	if err := tx.Commit(c.Context()); err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal menyimpan customer")
	}
	return utils.Success(c, fiber.StatusCreated, "Registrasi berhasil", fiber.Map{"id": userID, "name": req.Name, "email": req.Email, "role": "customer", "account_number": accountNumber})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h Handler) Login(c fiber.Ctx) error {
	var req loginRequest
	if err := c.Bind().Body(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "Request tidak valid")
	}
	var user struct{ ID, Name, Email, Password, Role, Status string }
	err := h.Pool.QueryRow(c.Context(), `SELECT id,name,email,password,role,status FROM users WHERE email=$1 AND deleted_at IS NULL`, strings.ToLower(strings.TrimSpace(req.Email))).Scan(&user.ID, &user.Name, &user.Email, &user.Password, &user.Role, &user.Status)
	if err != nil || user.Status != "active" || !utils.VerifyPassword(req.Password, user.Password) {
		return utils.Error(c, fiber.StatusUnauthorized, "Email atau password salah")
	}
	key, err := utils.EnsureBearerKey(c.Context(), h.Pool)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal menyiapkan token")
	}
	token, err := utils.CreateBearerToken(user.ID, user.Role, h.Cfg.AuthIssuer, key.PrivateKey, h.Cfg.TokenTTL)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal membuat token")
	}
	return utils.Success(c, fiber.StatusOK, "Login berhasil", fiber.Map{"access_token": token, "user": fiber.Map{"id": user.ID, "name": user.Name, "email": user.Email, "role": user.Role}})
}

type forgotRequest struct {
	Email string `json:"email"`
}

func (h Handler) ForgotPassword(c fiber.Ctx) error {
	var req forgotRequest
	if err := c.Bind().Body(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "Request tidak valid")
	}
	var userID, name, email string
	err := h.Pool.QueryRow(c.Context(), `SELECT id,name,email FROM users WHERE email=$1 AND deleted_at IS NULL`, strings.ToLower(strings.TrimSpace(req.Email))).Scan(&userID, &name, &email)
	if err == pgx.ErrNoRows {
		return utils.Success(c, fiber.StatusOK, "Jika email terdaftar, OTP akan dikirim", nil)
	}
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal memproses reset password")
	}
	otp, err := generateOTP()
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal membuat OTP")
	}
	otpHash, err := utils.HashPassword(otp, ArgonParams(h.Cfg))
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal mengamankan OTP")
	}
	id, _ := utils.NewCUID2()
	_, err = h.Pool.Exec(c.Context(), `INSERT INTO password_reset_otps (id,user_id,email,otp_hash,expires_at) VALUES ($1,$2,$3,$4,$5)`, id, userID, email, otpHash, time.Now().Add(h.Cfg.OTPExpiresIn))
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal menyimpan OTP")
	}
	if err := h.Email.SendOTP(c.Context(), email, name, otp); err != nil {
		return utils.Error(c, fiber.StatusBadGateway, "Gagal mengirim OTP ke email")
	}
	return utils.Success(c, fiber.StatusOK, "OTP berhasil dikirim ke email", fiber.Map{"otp_id": id})
}

type verifyOTPRequest struct {
	Email string `json:"email"`
	OTP   string `json:"otp"`
}

func (h Handler) VerifyOTP(c fiber.Ctx) error {
	var req verifyOTPRequest
	if err := c.Bind().Body(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "Request tidak valid")
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	var otpID, otpHash string
	var expiresAt time.Time
	err := h.Pool.QueryRow(c.Context(), `SELECT o.id,o.otp_hash,o.expires_at FROM password_reset_otps o JOIN users u ON u.id=o.user_id WHERE o.email=$1 AND o.verified_at IS NULL AND o.used_at IS NULL AND u.deleted_at IS NULL ORDER BY o.created_at DESC LIMIT 1`, email).Scan(&otpID, &otpHash, &expiresAt)
	if err != nil || time.Now().After(expiresAt) || !utils.VerifyPassword(req.OTP, otpHash) {
		return utils.Error(c, fiber.StatusBadRequest, "OTP tidak valid atau sudah kedaluwarsa")
	}
	resetToken, err := generateResetToken()
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal membuat token reset password")
	}
	resetTokenHash, err := utils.HashPassword(resetToken, ArgonParams(h.Cfg))
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal mengamankan token reset password")
	}
	_, err = h.Pool.Exec(c.Context(), `UPDATE password_reset_otps SET verified_at=now(), reset_token_hash=$1, reset_token_expires_at=$2 WHERE id=$3`, resetTokenHash, time.Now().Add(10*time.Minute), otpID)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal menandai OTP")
	}
	return utils.Success(c, fiber.StatusOK, "OTP valid", fiber.Map{"email": email, "reset_token": resetToken})
}

type resetPasswordRequest struct {
	Email          string `json:"email"`
	ResetToken     string `json:"reset_token"`
	Password       string `json:"password"`
	VerifyPassword string `json:"verify_password"`
}

func (h Handler) ResetPassword(c fiber.Ctx) error {
	var req resetPasswordRequest
	if err := c.Bind().Body(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "Request tidak valid")
	}
	if req.Password != req.VerifyPassword {
		return utils.Error(c, fiber.StatusBadRequest, "Konfirmasi password tidak sama")
	}
	if len(req.Password) < 8 {
		return utils.Error(c, fiber.StatusBadRequest, "Password minimal 8 karakter")
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	var otpID, userID, resetTokenHash string
	var resetTokenExpiresAt time.Time
	err := h.Pool.QueryRow(c.Context(), `SELECT o.id,o.user_id,o.reset_token_hash,o.reset_token_expires_at FROM password_reset_otps o JOIN users u ON u.id=o.user_id WHERE o.email=$1 AND o.verified_at IS NOT NULL AND o.used_at IS NULL AND o.reset_token_hash IS NOT NULL AND o.reset_token_expires_at IS NOT NULL AND u.deleted_at IS NULL ORDER BY o.created_at DESC LIMIT 1`, email).Scan(&otpID, &userID, &resetTokenHash, &resetTokenExpiresAt)
	if err != nil || time.Now().After(resetTokenExpiresAt) || !utils.VerifyPassword(req.ResetToken, resetTokenHash) {
		return utils.Error(c, fiber.StatusBadRequest, "Token reset password tidak valid atau sudah kedaluwarsa")
	}
	passwordHash, err := utils.HashPassword(req.Password, ArgonParams(h.Cfg))
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal mengenkripsi password")
	}
	tx, err := h.Pool.Begin(c.Context())
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal memulai transaksi database")
	}
	defer tx.Rollback(c.Context())
	_, err = tx.Exec(c.Context(), `UPDATE users SET password=$1, updated_at=now() WHERE id=$2`, passwordHash, userID)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal memperbarui password")
	}
	_, err = tx.Exec(c.Context(), `UPDATE password_reset_otps SET used_at=now() WHERE id=$1`, otpID)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal menandai token reset password")
	}
	if err := tx.Commit(c.Context()); err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Gagal menyimpan password baru")
	}
	return utils.Success(c, fiber.StatusOK, "Password berhasil diperbarui", nil)
}
