package orderbook

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type OrderBookShowSchema struct {
	Id             uuid.UUID       `json:"id" validate:"required"`
	AccountId      uuid.UUID       `json:"account_id" validate:"required"`
	InstrumentId   uuid.UUID       `json:"instrument_id" validate:"required"`
	Type           OrderType      `json:"type" validate:"required"`
	Status         OrderStatus    `json:"status" validate:"required"`
	Price          decimal.Decimal `json:"price" validate:"required"`
	TotalQuantity  decimal.Decimal `json:"total_quantity" validate:"required"`
	FilledQuantity decimal.Decimal `json:"filled_quantity" validate:"required"`
}

type PlaceOrderSchema struct {
	AccountId uuid.UUID       `json:"account_id" validate:"required"`
	AssetCode string          `json:"asset_code" validate:"required"`
	Quantity  decimal.Decimal `json:"quantity" validate:"required"`
	Price     decimal.Decimal `json:"price" validate:"required"`
	OrderType OrderType       `json:"order_type" validate:"required"`
}

type InstrumentWithAssetsSchema struct {
	Id             uuid.UUID `json:"id" validate:"required"`
	BaseAssetId    uuid.UUID `json:"base_asset_id" validate:"required"`
	BaseAssetCode  string    `json:"base_asset_code" validate:"required"`
	QuoteAssetId   uuid.UUID `json:"quote_asset_id" validate:"required"`
	QuoteAssetCode string    `json:"quote_asset_code" validate:"required"`
}
