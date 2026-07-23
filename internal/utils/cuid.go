package utils

import "github.com/nrednav/cuid2"

func NewCUID2() (string, error) {
	return cuid2.Generate(), nil
}

func MustCUID2() string {
	id, err := NewCUID2()
	if err != nil {
		panic(err)
	}
	return id
}
