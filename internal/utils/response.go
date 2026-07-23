package utils

import "github.com/gofiber/fiber/v3"

type APIResponse struct {
	Data    any    `json:"data,omitempty"`
	Status  bool   `json:"status"`
	Message string `json:"message"`
}

func Success(c fiber.Ctx, code int, message string, data any) error {
	return c.Status(code).JSON(APIResponse{Data: data, Status: true, Message: message})
}

func Error(c fiber.Ctx, code int, message string) error {
	return c.Status(code).JSON(APIResponse{Status: false, Message: message})
}
