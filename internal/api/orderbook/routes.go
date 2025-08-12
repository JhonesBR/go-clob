package orderbook

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
)

func InitializeRoutes(app *fiber.App, db *pgxpool.Pool) {
	app.Get("/v1/order_book", GetOrderBookHandler(db))
	app.Post("/v1/order_book", PlaceOrderHandler(context.Background(), db))
	app.Post("/v1/order_book/:id/cancel", CancelOrderHandler(context.Background(), db))
}
