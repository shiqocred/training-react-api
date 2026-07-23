package utils

import (
	"math"
	"strconv"

	"github.com/gofiber/fiber/v3"
)

type Pagination struct {
	CurrentPage int   `json:"current_page"`
	PerPage     int   `json:"per_page"`
	From        int   `json:"from"`
	Total       int64 `json:"total"`
	LastPage    int   `json:"last_page"`
}

type PaginatedData struct {
	Items      any        `json:"items"`
	Pagination Pagination `json:"pagination"`
}

func GetPagination(c fiber.Ctx) (page int, perPage int, offset int) {
	page = parsePositiveInt(c.Query("page", "1"), 1)
	perPage = parsePositiveInt(c.Query("per_page", "10"), 10)
	if perPage > 100 {
		perPage = 100
	}
	offset = (page - 1) * perPage
	return
}

func NewPagination(page, perPage int, total int64) Pagination {
	from := 0
	if total > 0 {
		from = ((page - 1) * perPage) + 1
	}
	lastPage := int(math.Ceil(float64(total) / float64(perPage)))
	if lastPage == 0 {
		lastPage = 1
	}
	return Pagination{CurrentPage: page, PerPage: perPage, From: from, Total: total, LastPage: lastPage}
}

func parsePositiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}
