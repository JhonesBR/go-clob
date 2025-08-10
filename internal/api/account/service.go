package account

import (
	"context"
	"fmt"

	"github.com/JhonesBR/go-clob/internal/helper"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

func CreateNewAccount(db *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Parse create account schema
		var account = CreateAccountSchema{}
		if err := c.Bind().Body(&account); err != nil {
			return fiber.ErrBadRequest
		}
		if err := helper.ValidateInput(&account); err != nil {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		// Create a new account at database
		accountId := uuid.New()

		err := db.QueryRow(context.Background(), "INSERT INTO accounts (id, balance) VALUES ($1, $2) RETURNING id, balance", accountId, account.Balance).Scan(&accountId, &account.Balance)
		if err != nil {
			return err
		}

		return c.JSON(CreateAccountResponseSchema{
			Id:      accountId.String(),
			Balance: *account.Balance,
		})
	}
}

func GetAccounts(db *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Get pagination
		pagination := helper.GetPagination[AccountShowSchema](c)

		// Get total
		count_query := "SELECT COUNT(*) FROM accounts"
		var total int
		if err := db.QueryRow(context.Background(), count_query).Scan(&total); err != nil {
			return err
		}
		pagination.Total = &total

		// Retrieve accounts
		query := fmt.Sprintf("SELECT id, balance FROM accounts LIMIT %d OFFSET %d", pagination.Size, (pagination.Page-1)*pagination.Size)
		rows, err := db.Query(context.Background(), query)
		if err != nil {
			return err
		}
		defer rows.Close()

		var accounts []AccountShowSchema
		for rows.Next() {
			var account AccountShowSchema
			if err := rows.Scan(&account.Id, &account.Balance); err != nil {
				return err
			}
			accounts = append(accounts, account)
		}

		pagination.Items = accounts

		return c.JSON(pagination)
	}
}

func GetAccountByID(db *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		id := c.Params("id")
		if id == "" {
			return fiber.ErrBadRequest
		}

		// Get account
		var account AccountShowSchema
		err := db.QueryRow(context.Background(), "SELECT id, balance FROM accounts WHERE id = $1", id).Scan(&account.Id, &account.Balance)
		if err != nil {
			if err == pgx.ErrNoRows {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Account not found"})
			}
			return err
		}

		return c.JSON(account)
	}
}

func UpdateAccountBalance(ctx context.Context, db *pgxpool.Pool, operation string) fiber.Handler {
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

		// Get account balance
		balance, err := getAccountBalance(ctx, db, id)
		if err != nil {
			return err
		}

		// Charge or remove balance
		var charge UpdateBalanceSchema
		if err := c.Bind().Body(&charge); err != nil {
			return fiber.ErrBadRequest
		}
		if err := helper.ValidateInput(&charge); err != nil {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		switch operation {
		case "charge":
			balance = balance.Add(*charge.Amount)
		case "remove":
			balance = balance.Sub(*charge.Amount)
		}

		updateAccountBalance(ctx, db, id, balance)

		// Commit transaction
		if err := tx.Commit(ctx); err != nil {
			return err
		}

		return c.JSON(UpdateBalanceResponseSchema{
			Balance: balance,
		})
	}
}

func getAccountBalance(ctx context.Context, db *pgxpool.Pool, id string) (decimal.Decimal, error) {
	var balance decimal.Decimal
	err := db.QueryRow(ctx, "SELECT balance FROM accounts WHERE id = $1 FOR UPDATE", id).Scan(&balance)
	if err != nil {
		if err == pgx.ErrNoRows {
			return decimal.Decimal{}, fmt.Errorf("account not found")
		}
		return decimal.Decimal{}, err
	}
	return balance, nil
}

func updateAccountBalance(ctx context.Context, db *pgxpool.Pool, id string, newBalance decimal.Decimal) error {
	_, err := db.Exec(ctx, "UPDATE accounts SET balance = $1 WHERE id = $2", newBalance, id)
	return err
}
