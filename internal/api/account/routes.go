package account

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
)

func InitializeRoutes(app *fiber.App, db *pgxpool.Pool) {
	app.Get("/v1/accounts", GetAccountsHandler(db))
	app.Post("/v1/accounts", CreateNewAccountHandler(db))
	app.Get("/v1/accounts/:id", GetAccountByIDHandler(db))
	app.Post("/v1/accounts/:id/charge", UpdateAccountBalanceHandler(context.Background(), db, "charge"))
	app.Post("/v1/accounts/:id/remove", UpdateAccountBalanceHandler(context.Background(), db, "remove"))
}
