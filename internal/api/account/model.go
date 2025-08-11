package account

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Representative (schemas will be used for validation and documentation)

type Asset struct {
	Id   uuid.UUID `json:"id"`
	Code string    `json:"code"`
	Name string    `json:"name"`
}

type Account struct {
	Id   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type AccountBalance struct {
	Id        uuid.UUID       `json:"id"`
	AccountId uuid.UUID       `json:"account_id"`
	AssetId   uuid.UUID       `json:"asset_id"`
	Balance   decimal.Decimal `json:"balance"`
}
