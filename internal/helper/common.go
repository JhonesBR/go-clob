package helper

import (
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
)

type Pagination[T any] struct {
	Page  int  `json:"page"`
	Size  int  `json:"size"`
	Total *int `json:"total"`
	Items []T  `json:"items"`
}

func GetPagination[T any](c fiber.Ctx) Pagination[T] {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	if page < 1 {
		page = 1
	}

	size, _ := strconv.Atoi(c.Query("size", "50"))
	if size < 1 {
		size = 1
	} else if size > 100 {
		size = 100
	}

	return Pagination[T]{
		Page:  page,
		Size:  size,
		Total: nil,
		Items: []T{},
	}
}

var validate = validator.New()

func ValidateInput(input interface{}) error {
	return validate.Struct(input)
}

func MapToSlice[T any](m map[string]T) []T {
	var slice []T
	for _, v := range m {
		slice = append(slice, v)
	}
	return slice
}
