package utils

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
)

const EndpointLogIDKey = "endpoint_log_id"

type EndpointLogUpdate struct {
	UserID     *string
	Response   any
	StatusCode int
	Success    bool
	Error      bool
	ResponseAt time.Time
}

func InsertEndpointLog(ctx context.Context, pool *pgxpool.Pool, c fiber.Ctx) (string, error) {
	id, err := NewCUID2()
	if err != nil {
		return "", err
	}

	request := map[string]any{
		"method":      c.Method(),
		"url":         c.OriginalURL(),
		"query":       cloneStringMap(c.Queries()),
		"headers":     sanitizeHeaders(c.GetReqHeaders()),
		"body":        ParseJSONForLog(c.Body()),
		"request_at":  time.Now().Format(time.RFC3339Nano),
		"remote_addr": c.IP(),
	}
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return "", err
	}

	_, err = pool.Exec(ctx, `INSERT INTO endpoint_logs (id, method, endpoint, request, request_at) VALUES ($1,$2,$3,$4,now())`, id, c.Method(), c.OriginalURL(), string(requestJSON))
	if err != nil {
		return "", err
	}
	c.Locals(EndpointLogIDKey, id)
	return id, nil
}

func UpdateEndpointLog(ctx context.Context, pool *pgxpool.Pool, logID string, update EndpointLogUpdate) error {
	if logID == "" {
		return nil
	}
	if update.ResponseAt.IsZero() {
		update.ResponseAt = time.Now()
	}
	responseJSON, err := json.Marshal(update.Response)
	if err != nil {
		responseJSON = []byte(`{"message":"response tidak dapat dikonversi ke JSON"}`)
	}
	_, err = pool.Exec(ctx, `
		UPDATE endpoint_logs
		SET user_id = COALESCE($2, user_id),
		    response = $3,
		    status_code = $4,
		    success = $5,
		    error = $6,
		    response_at = $7,
		    duration_ms = (EXTRACT(EPOCH FROM ($7 - request_at)) * 1000)::BIGINT
		WHERE id = $1
	`, logID, update.UserID, string(responseJSON), update.StatusCode, update.Success, update.Error, update.ResponseAt)
	return err
}

func EndpointLogID(c fiber.Ctx) string {
	id, _ := c.Locals(EndpointLogIDKey).(string)
	return id
}

func LogEndpointError(ctx context.Context, pool *pgxpool.Pool, logID string, statusCode int, message string) error {
	return UpdateEndpointLog(ctx, pool, logID, EndpointLogUpdate{
		Response:   map[string]any{"status": false, "message": message},
		StatusCode: statusCode,
		Success:    false,
		Error:      true,
	})
}

func cloneStringMap(input map[string]string) map[string]string {
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func sanitizeHeaders(headers map[string][]string) map[string][]string {
	output := make(map[string][]string, len(headers))
	for key, values := range headers {
		if strings.EqualFold(key, "authorization") || strings.EqualFold(key, "cookie") {
			output[key] = []string{"[REDACTED]"}
			continue
		}
		copied := make([]string, len(values))
		copy(copied, values)
		output[key] = copied
	}
	return output
}

func ParseJSONForLog(body []byte) any {
	if len(body) == 0 {
		return nil
	}
	var decoded any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return string(body)
	}
	return decoded
}
