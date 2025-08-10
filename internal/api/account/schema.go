package account

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type CreateAccountSchema struct {
	Name string `json:"name" validate:"required"`
}

type AccountBalanceSchema struct {
	AssetId   *uuid.UUID       `json:"asset_id" validate:"required"`
	Balance   *decimal.Decimal `json:"balance" validate:"required"`
	AssetCode *string          `json:"asset_code" validate:"required"`
}

type AccountShowSchema struct {
	Id       string                 `json:"id" validate:"required"`
	Name     string                 `json:"name" validate:"required"`
	Balances []AccountBalanceSchema `json:"balances" validate:"required"`
}

type CreateAccountResponseSchema = AccountShowSchema

type UpdateBalanceSchema struct {
	Amount    *decimal.Decimal `json:"amount" validate:"required"`
	AssetCode *string          `json:"asset_code" validate:"required"`
}

type UpdateBalanceResponseSchema struct {
	Balance   *decimal.Decimal `json:"balance" validate:"required"`
	AssetCode *string          `json:"asset_code" validate:"required"`
}
