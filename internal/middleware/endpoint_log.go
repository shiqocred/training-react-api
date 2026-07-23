package middleware

import (
	"time"

	"react-api/internal/utils"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
)

func EndpointLogger(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		logID, insertErr := utils.InsertEndpointLog(c.Context(), pool, c)
		if insertErr != nil {
			return c.Next()
		}

		err := c.Next()
		statusCode := fiber.StatusInternalServerError
		response := map[string]any{
			"status":  false,
			"message": "Terjadi kesalahan pada server",
		}

		if resp := c.Response(); resp != nil {
			statusCode = resp.StatusCode()
			response = map[string]any{
				"headers":     resp.Header.String(),
				"body":        parseResponseBody(resp.Body()),
				"response_at": time.Now().Format(time.RFC3339Nano),
			}
		}

		success := statusCode >= 200 && statusCode < 400 && err == nil
		isError := !success
		var userID *string
		if auth, ok := c.Locals("auth").(AuthContext); ok && auth.UserID != "" {
			userID = &auth.UserID
		}

		_ = utils.UpdateEndpointLog(c.Context(), pool, logID, utils.EndpointLogUpdate{
			UserID:     userID,
			Response:   response,
			StatusCode: statusCode,
			Success:    success,
			Error:      isError,
			ResponseAt: time.Now(),
		})
		return err
	}
}

func parseResponseBody(body []byte) any {
	return utils.ParseJSONForLog(body)
}
