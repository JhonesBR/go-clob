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
		balance, assetId, err := account.GetAccountBalance(ctx, tx, order.AccountId, assetCode)
		if err != nil {
			if err == pgx.ErrNoRows {
				return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
					"error": "Insufficient funds",
				})
			}
			return err
		}
		if balance.LessThan(order.Quantity.Mul(order.Price)) {
			return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
				"error": "Insufficient funds",
			})
		}

		// Update balance from account
		if err := account.UpdateAccountBalance(ctx, tx, order.AccountId, balance.Sub(order.Quantity.Mul(order.Price)), *assetId); err != nil {
			return err
		}

		// Create a new order at database
		query := `
			INSERT INTO order_book (account_id, instrument_id, type, status, price, total_quantity, filled_quantity)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`
		tx.Exec(
			context.Background(),
			query,
			order.AccountId,
			instrument.Id,
			order.OrderType,
			Open,
			order.Price,
			order.Quantity,
			0,
		)

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
