package account

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Account struct {
	Id      uuid.UUID       `json:"id"`
	Balance decimal.Decimal `json:"balance"`
}
