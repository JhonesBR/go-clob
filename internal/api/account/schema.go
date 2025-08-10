package account

import (
	"github.com/shopspring/decimal"
)

type CreateAccountSchema struct {
	Balance *decimal.Decimal `json:"balance" validate:"required"`
}

type CreateAccountResponseSchema struct {
	Id      string          `json:"id" validate:"required"`
	Balance decimal.Decimal `json:"balance" validate:"required"`
}

type AccountShowSchema struct {
	Id      string          `json:"id" validate:"required"`
	Balance decimal.Decimal `json:"balance" validate:"required"`
}

type UpdateBalanceSchema struct {
	Amount *decimal.Decimal `json:"amount" validate:"required"`
}

type UpdateBalanceResponseSchema struct {
	Balance decimal.Decimal `json:"balance" validate:"required"`
}
