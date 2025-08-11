package orderbook

import (
	"context"
	"fmt"

	"github.com/JhonesBR/go-clob/internal/api/account"
	"github.com/JhonesBR/go-clob/internal/helper"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

func PlaceOrderHandler(ctx context.Context, db *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Parse place order schema
		var order = PlaceOrderSchema{}
		if err := c.Bind().Body(&order); err != nil {
			return fiber.ErrBadRequest
		}
		if err := helper.ValidateInput(&order); err != nil {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		// Transaction to ensure correct update on race conditions
		tx, err := db.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)

		// Get instrument of order
		instrument, err := getInstrumentByAssetCode(ctx, tx, order.AssetCode)
		if err != nil {
			if err == pgx.ErrNoRows {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
					"error": "Instrument not found",
				})
			}
			return err
		}

		var assetCode string
		if order.OrderType == Buy {
			assetCode = instrument.QuoteAssetCode
		} else {
			assetCode = order.AssetCode
		}

		// Verify if the account has the balance
		balance, assetId, err := account.GetAccountBalance(ctx, tx, order.AccountId, &assetCode, nil)
		if err != nil {
			if err == pgx.ErrNoRows {
				return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
					"error": "Insufficient funds",
				})
			}
			return err
		}

		// Verify if the account has the necessary balance
		var necessaryBalance decimal.Decimal
		if order.OrderType == Buy {
			necessaryBalance = order.Quantity.Mul(order.Price)
		} else {
			necessaryBalance = order.Quantity
		}
		if balance.LessThan(necessaryBalance) {
			return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
				"error": "Insufficient funds",
			})
		}

		// Update balance from account
		var newBalance decimal.Decimal
		if order.OrderType == Buy {
			newBalance = balance.Sub(order.Quantity.Mul(order.Price))
		} else {
			newBalance = balance.Sub(order.Quantity)
		}
		if err := account.UpdateAccountBalance(ctx, tx, order.AccountId, newBalance, *assetId); err != nil {
			return err
		}

		// Create a new order at database
		var orderId uuid.UUID
		query := `
			INSERT INTO order_book (account_id, instrument_id, type, status, price, total_quantity, filled_quantity)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id
		`
		err = tx.QueryRow(
			context.Background(),
			query,
			order.AccountId,
			instrument.Id,
			order.OrderType,
			Open,
			order.Price,
			order.Quantity,
			0,
		).Scan(&orderId)
		if err != nil {
			return err
		}

		// Match order
		orderToMatch := OrderBook{
			Id:             orderId,
			AccountId:      order.AccountId,
			InstrumentId:   instrument.Id,
			Type:           order.OrderType,
			Status:         Open,
			Price:          order.Price,
			TotalQuantity:  order.Quantity,
			FilledQuantity: decimal.NewFromInt(0),
		}
		if err := matchOrder(ctx, tx, orderToMatch, instrument); err != nil {
			return err
		}

		// Commit transaction
		if err := tx.Commit(ctx); err != nil {
			return err
		}

		return c.SendStatus(fiber.StatusNoContent)
	}
}

func CancelOrderHandler(ctx context.Context, db *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		id := c.Params("id")
		if id == "" {
			return fiber.ErrBadRequest
		}

		// Transaction to ensure correct update on race conditions
		tx, err := db.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)

		// Get order
		var order OrderBook
		query := `
			SELECT id, account_id, instrument_id, type, status, price, total_quantity, filled_quantity
			FROM order_book
			WHERE id = $1
			FOR UPDATE
		`
		if err := tx.QueryRow(ctx, query, id).Scan(&order.Id, &order.AccountId, &order.InstrumentId, &order.Type, &order.Status, &order.Price, &order.TotalQuantity, &order.FilledQuantity); err != nil {
			if err == pgx.ErrNoRows {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
					"error": "Order not found",
				})
			}
			return err
		}

		// Verify eligibility for cancelation
		if err := verifyOrderCancelationEligibility(order); err != nil {
			return c.JSON(fiber.Map{
				"error": fmt.Sprintf("order is not eligible for cancelation (reason: %s)", err.Error()),
			})
		}

		// Get asset of order
		var asset account.Asset
		var innerJoin string
		if order.Type == Buy {
			innerJoin = "INNER JOIN instruments ON instruments.quote_asset_id = assets.id"
		} else {
			innerJoin = "INNER JOIN instruments ON instruments.base_asset_id = assets.id"
		}

		query = `
			SELECT assets.id, assets.code
			FROM assets
			` + innerJoin + `
			WHERE instruments.id = $1
			FOR UPDATE
		`
		if err := tx.QueryRow(ctx, query, order.InstrumentId).Scan(&asset.Id, &asset.Code); err != nil {
			if err == pgx.ErrNoRows {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
					"error": "Asset not found",
				})
			}
			return err
		}

		// Update order status
		if err := updateOrderStatus(ctx, tx, order.Id, Canceled); err != nil {
			return err
		}

		// Rollback account balance
		if err := account.UpdateAccountBalance(ctx, tx, order.AccountId, order.TotalQuantity.Sub(order.FilledQuantity), asset.Id); err != nil {
			return err
		}

		// Commit transaction
		if err := tx.Commit(ctx); err != nil {
			return err
		}

		return c.SendStatus(fiber.StatusNoContent)
	}
}

func getInstrumentByAssetCode(ctx context.Context, tx pgx.Tx, assetCode string) (InstrumentWithAssetsSchema, error) {
	var instrument InstrumentWithAssetsSchema
	query := `
		SELECT instruments.id, instruments.base_asset_id, base_assets.code, instruments.quote_asset_id, quote_assets.code
		FROM instruments
		INNER JOIN assets base_assets ON base_assets.id = instruments.base_asset_id
		INNER JOIN assets quote_assets ON quote_assets.id = instruments.quote_asset_id
		WHERE base_assets.code = $1
	`
	err := tx.QueryRow(ctx, query, assetCode).Scan(&instrument.Id, &instrument.BaseAssetId, &instrument.BaseAssetCode, &instrument.QuoteAssetId, &instrument.QuoteAssetCode)
	if err != nil {
		return InstrumentWithAssetsSchema{}, err
	}
	return instrument, nil
}

func verifyOrderCancelationEligibility(order OrderBook) error {
	// Order need to be in status open or partially filled
	if order.Status != Open && order.Status != PartiallyFilled {
		return fmt.Errorf("order need to be open or partially filled")
	}

	return nil
}

func updateOrderStatus(ctx context.Context, tx pgx.Tx, orderId uuid.UUID, status OrderStatus) error {
	_, err := tx.Exec(ctx, "UPDATE order_book SET status = $1 WHERE id = $2", status, orderId)
	return err
}

func matchOrder(ctx context.Context, tx pgx.Tx, order OrderBook, instrument InstrumentWithAssetsSchema) error {
	// Get matches for buy/sell order
	var matchOrders []OrderBook
	var err error
	if order.Type == Buy {
		matchOrders, err = getCompatibleSellOrders(ctx, tx, order)
		if err != nil {
			return err
		}
	} else {
		matchOrders, err = getCompatibleBuyOrders(ctx, tx, order)
		if err != nil {
			return err
		}
	}

	for _, match := range matchOrders {
		order, err = processMatch(ctx, tx, order, match, instrument)
		if err != nil {
			return err
		}
	}

	return nil
}

func getCompatibleSellOrders(ctx context.Context, tx pgx.Tx, order OrderBook) ([]OrderBook, error) {
	var compatibleOrders []OrderBook

	query := `
		SELECT id, account_id, instrument_id, type, status, price, total_quantity, filled_quantity
		FROM order_book
		WHERE
			instrument_id = $1
			AND type = 'sell'
			AND status IN ('open', 'partially_filled')
			AND price <= $2
		ORDER BY
			price ASC,
			created_at ASC
		FOR UPDATE SKIP LOCKED
	`
	rows, err := tx.Query(ctx, query, order.InstrumentId, order.Price)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var compatibleOrder OrderBook
		if err := rows.Scan(&compatibleOrder.Id, &compatibleOrder.AccountId, &compatibleOrder.InstrumentId, &compatibleOrder.Type, &compatibleOrder.Status, &compatibleOrder.Price, &compatibleOrder.TotalQuantity, &compatibleOrder.FilledQuantity); err != nil {
			return nil, err
		}
		compatibleOrders = append(compatibleOrders, compatibleOrder)
	}

	return compatibleOrders, nil
}

func getCompatibleBuyOrders(ctx context.Context, tx pgx.Tx, order OrderBook) ([]OrderBook, error) {
	var compatibleOrders []OrderBook

	query := `
		SELECT id, account_id, instrument_id, type, status, price, total_quantity, filled_quantity
		FROM order_book
		WHERE
			instrument_id = $1
			AND type = 'buy'
			AND status IN ('open', 'partially_filled')
			AND price >= $2
		ORDER BY
			price DESC,
			created_at ASC
		FOR UPDATE SKIP LOCKED
	`
	rows, err := tx.Query(ctx, query, order.InstrumentId, order.Price)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var compatibleOrder OrderBook
		if err := rows.Scan(&compatibleOrder.Id, &compatibleOrder.AccountId, &compatibleOrder.InstrumentId, &compatibleOrder.Type, &compatibleOrder.Status, &compatibleOrder.Price, &compatibleOrder.TotalQuantity, &compatibleOrder.FilledQuantity); err != nil {
			return nil, err
		}
		compatibleOrders = append(compatibleOrders, compatibleOrder)
	}

	return compatibleOrders, nil
}

func processMatch(ctx context.Context, tx pgx.Tx, order OrderBook, match OrderBook, instrument InstrumentWithAssetsSchema) (OrderBook, error) {
	// Split orders into buy and sell
	var buyOrder, sellOrder OrderBook
	if order.Type == Buy {
		buyOrder = order
		sellOrder = match
	} else if order.Type == Sell && match.Type == Buy {
		buyOrder = match
		sellOrder = order
	}

	buyOrderAvailableQuantity := buyOrder.TotalQuantity.Sub(buyOrder.FilledQuantity)
	sellOrderAvailableQuantity := sellOrder.TotalQuantity.Sub(sellOrder.FilledQuantity)

	// Determine the quantity to fulfill
	fullfillQuantity := decimal.Min(buyOrderAvailableQuantity, sellOrderAvailableQuantity)

	// Fill each order
	buyOrderNewFilledQuantity := buyOrder.FilledQuantity.Add(fullfillQuantity)
	if err := fillOrder(ctx, tx, buyOrder, buyOrderNewFilledQuantity); err != nil {
		return OrderBook{}, err
	}
	sellOrderNewFilledQuantity := sellOrder.FilledQuantity.Add(fullfillQuantity)
	if err := fillOrder(ctx, tx, sellOrder, sellOrderNewFilledQuantity); err != nil {
		return OrderBook{}, err
	}

	// Update orders statuses
	if buyOrderNewFilledQuantity.GreaterThan(decimal.NewFromInt(0)) {
		if buyOrderNewFilledQuantity.LessThan(buyOrder.TotalQuantity) {
			if err := updateOrderStatus(ctx, tx, buyOrder.Id, PartiallyFilled); err != nil {
				return OrderBook{}, err
			}
		} else {
			if err := updateOrderStatus(ctx, tx, buyOrder.Id, FullFilled); err != nil {
				return OrderBook{}, err
			}
		}
	}
	if sellOrderNewFilledQuantity.GreaterThan(decimal.NewFromInt(0)) {
		if sellOrderNewFilledQuantity.LessThan(sellOrder.TotalQuantity) {
			if err := updateOrderStatus(ctx, tx, sellOrder.Id, PartiallyFilled); err != nil {
				return OrderBook{}, err
			}
		} else {
			if err := updateOrderStatus(ctx, tx, sellOrder.Id, FullFilled); err != nil {
				return OrderBook{}, err
			}
		}
	}

	// Charge the buy account with the asset
	accountBalance, _, err := account.GetAccountBalance(ctx, tx, order.AccountId, nil, &instrument.BaseAssetId)
	if err != nil {
		return OrderBook{}, err
	}
	if accountBalance == nil {
		err = account.CreateAccountBalanceForAccount(ctx, tx, order.AccountId, instrument.BaseAssetId)
		if err != nil {
			return OrderBook{}, err
		}
		accountBalance = &decimal.Decimal{}
	}
	newAccountBalance := accountBalance.Add(fullfillQuantity)
	if err := account.UpdateAccountBalance(ctx, tx, order.AccountId, newAccountBalance, instrument.BaseAssetId); err != nil {
		return OrderBook{}, err
	}

	// Charge the sell account with the quote asset
	sellAccountBalance, _, err := account.GetAccountBalance(ctx, tx, sellOrder.AccountId, nil, &instrument.QuoteAssetId)
	if err != nil {
		return OrderBook{}, err
	}
	if sellAccountBalance == nil {
		err = account.CreateAccountBalanceForAccount(ctx, tx, sellOrder.AccountId, instrument.QuoteAssetId)
		if err != nil {
			return OrderBook{}, err
		}
		sellAccountBalance = &decimal.Decimal{}
	}
	newSellAccountBalance := sellAccountBalance.Add(fullfillQuantity.Mul(sellOrder.Price))
	if err := account.UpdateAccountBalance(ctx, tx, sellOrder.AccountId, newSellAccountBalance, instrument.QuoteAssetId); err != nil {
		return OrderBook{}, err
	}

	// Update current order to return
	if order.Type == Buy {
		order.FilledQuantity = buyOrderNewFilledQuantity
	} else {
		order.FilledQuantity = sellOrderNewFilledQuantity
	}

	return order, nil
}

func fillOrder(ctx context.Context, tx pgx.Tx, order OrderBook, quantity decimal.Decimal) error {
	_, err := tx.Exec(ctx, "UPDATE order_book SET filled_quantity = filled_quantity + $1 WHERE id = $2", quantity, order.Id)
	return err
}
