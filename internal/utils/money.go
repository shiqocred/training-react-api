package utils

import (
	"errors"
	"strconv"
	"strings"
)

func ParseAmountToCents(input string) (int64, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return 0, errors.New("jumlah wajib diisi")
	}
	parts := strings.Split(input, ".")
	if len(parts) > 2 {
		return 0, errors.New("format jumlah tidak valid")
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole < 0 {
		return 0, errors.New("jumlah tidak valid")
	}
	cents := int64(0)
	if len(parts) == 2 {
		fraction := parts[1]
		if len(fraction) > 2 {
			return 0, errors.New("jumlah maksimal memiliki 2 angka desimal")
		}
		for len(fraction) < 2 {
			fraction += "0"
		}
		cents, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, errors.New("jumlah tidak valid")
		}
	}
	amount := whole*100 + cents
	if amount <= 0 {
		return 0, errors.New("jumlah harus lebih dari 0")
	}
	return amount, nil
}
