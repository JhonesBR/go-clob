package orderbook

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Representative (schemas will be used for validation and documentation)

type Instrument struct {
	Id           uuid.UUID `json:"id"`
	BaseAssetId  uuid.UUID `json:"base_asset_id"`
	QuoteAssetId uuid.UUID `json:"quote_asset_id"`
}

type OrderType string

const (
	Buy  OrderType = "buy"
	Sell OrderType = "sell"
)

type OrderStatus string

const (
	Open            OrderStatus = "open"
	PartiallyFilled OrderStatus = "partially_filled"
	FullFilled      OrderStatus = "full_filled"
	Canceled        OrderStatus = "canceled"
)

type OrderBook struct {
	Id               uuid.UUID       `json:"id"`
	AccountId        uuid.UUID       `json:"account_id"`
	InstrumentId     uuid.UUID       `json:"instrument_id"`
	Type             OrderType       `json:"type"`
	Status           OrderStatus     `json:"status"`
	Price            decimal.Decimal `json:"price"`
	TotalQuantity    decimal.Decimal `json:"total_quantity"`
	FilledQuantity   decimal.Decimal `json:"filled_quantity"`
	CreatedAt        string          `json:"created_at"`
}
