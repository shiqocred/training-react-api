package auth

import (
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"

	"react-api/internal/config"
	"react-api/internal/utils"
)

var emailRegex = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

func validateNameEmailPassword(name, email, password string) string {
	if strings.TrimSpace(name) == "" {
		return "Nama wajib diisi"
	}
	if !emailRegex.MatchString(strings.ToLower(strings.TrimSpace(email))) {
		return "Email tidak valid"
	}
	if len(password) < 8 {
		return "Password minimal 8 karakter"
	}
	return ""
}

func ArgonParams(cfg config.Config) utils.ArgonParams {
	return utils.ArgonParams{Memory: cfg.ArgonMemory, Iterations: cfg.ArgonIterations, Parallelism: cfg.ArgonParallelism, SaltLength: cfg.ArgonSaltLength, KeyLength: cfg.ArgonKeyLength}
}

func generateOTP() (string, error) {
	var b [1]byte
	otp := ""
	for len(otp) < 6 {
		if _, err := rand.Read(b[:]); err != nil {
			return "", err
		}
		otp += fmt.Sprintf("%d", int(b[0])%10)
	}
	return otp, nil
}
