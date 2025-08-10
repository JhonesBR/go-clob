package account

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
)

func InitializeRoutes(app *fiber.App, db *pgxpool.Pool) {
	app.Get("/v1/accounts", GetAccounts(db))
	app.Post("/v1/accounts", CreateNewAccount(db))
	app.Get("/v1/accounts/:id", GetAccountByID(db))
	app.Post("/v1/accounts/:id/charge", UpdateAccountBalance(context.Background(), db, "charge"))
	app.Post("/v1/accounts/:id/remove", UpdateAccountBalance(context.Background(), db, "remove"))
}
