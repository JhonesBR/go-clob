package api

import (
	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JhonesBR/go-clob/internal/api/account"
	"github.com/JhonesBR/go-clob/internal/api/orderbook"
)

func InitializeRoutes(app *fiber.App, db *pgxpool.Pool) {
	account.InitializeRoutes(app, db)
	orderbook.InitializeRoutes(app, db)
}
