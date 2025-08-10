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
		var accountId uuid.UUID
		err := db.QueryRow(context.Background(), "INSERT INTO accounts (name) VALUES ($1) RETURNING id", account.Name).Scan(&accountId)
		if err != nil {
			return err
		}

		return c.JSON(CreateAccountResponseSchema{
			Id:       accountId.String(),
			Name:     account.Name,
			Balances: []AccountBalanceSchema{},
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
		query := fmt.Sprintf(
			`SELECT acc.id, acc.name, ab.balance, assets.code, assets.id
			 FROM accounts acc
			 LEFT JOIN account_balances ab ON acc.id = ab.account_id
			 LEFT JOIN assets ON ab.asset_id = assets.id
			 LIMIT %d OFFSET %d`,
			pagination.Size,
			(pagination.Page-1)*pagination.Size,
		)
		rows, err := db.Query(context.Background(), query)
		if err != nil {
			return err
		}
		defer rows.Close()

		var accounts = make(map[string]AccountShowSchema)
		for rows.Next() {
			var account AccountShowSchema
			var balance *decimal.Decimal
			var assetCode *string
			var assetId *uuid.UUID
			if err := rows.Scan(&account.Id, &account.Name, &balance, &assetCode, &assetId); err != nil {
				return err
			}

			if _, ok := accounts[account.Id]; !ok {
				accounts[account.Id] = AccountShowSchema{
					Id:       account.Id,
					Name:     account.Name,
					Balances: make([]AccountBalanceSchema, 0),
				}
			}

			if balance != nil && assetCode != nil {
				accountData := accounts[account.Id]
				accountData.Balances = append(accountData.Balances, AccountBalanceSchema{
					AssetId:   assetId,
					Balance:   balance,
					AssetCode: assetCode,
				})
				accounts[account.Id] = accountData
			}
		}

		pagination.Items = helper.MapToSlice(accounts)

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
		query := fmt.Sprintf(`
			SELECT acc.id, acc.name, ab.balance, assets.code, assets.id
			 FROM accounts acc
			 LEFT JOIN account_balances ab ON acc.id = ab.account_id
			 LEFT JOIN assets ON ab.asset_id = assets.id
			 WHERE acc.id = '%s'
		`, id)

		rows, err := db.Query(context.Background(), query)
		if err != nil {
			return err
		}
		defer rows.Close()

		account.Balances = make([]AccountBalanceSchema, 0)
		for rows.Next() {
			var balance AccountBalanceSchema
			if err := rows.Scan(&account.Id, &account.Name, &balance.Balance, &balance.AssetCode, &balance.AssetId); err != nil {
				return err
			}

			if balance.Balance != nil {
				account.Balances = append(account.Balances, balance)
			}
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

		// Get account balance
		balance, assetId, err := getAccountBalance(ctx, db, id, *charge.AssetCode)
		if err != nil {
			if err == pgx.ErrNoRows {
				createAccountBalanceForAccount(ctx, db, id, *assetId)
			} else {
				return err
			}
		}

		switch operation {
		case "charge":
			balance = balance.Add(*charge.Amount)
		case "remove":
			balance = balance.Sub(*charge.Amount)
		}

		updateAccountBalance(ctx, db, id, balance, *assetId)

		// Commit transaction
		if err := tx.Commit(ctx); err != nil {
			return err
		}

		return c.JSON(UpdateBalanceResponseSchema{
			Balance:   &balance,
			AssetCode: charge.AssetCode,
		})
	}
}

func getAccountBalance(ctx context.Context, db *pgxpool.Pool, accountId, assetCode string) (decimal.Decimal, *uuid.UUID, error) {
	var assetId uuid.UUID
	err := db.QueryRow(ctx, "SELECT id FROM assets WHERE code = $1", assetCode).Scan(&assetId)
	if err != nil {
		if err == pgx.ErrNoRows {
			return decimal.NewFromInt(0), nil, fmt.Errorf("asset not found")
		}
		return decimal.Decimal{}, nil, err
	}

	var balance decimal.Decimal
	query := `
		SELECT ab.balance, assets.id
		FROM assets
		LEFT OUTER JOIN account_balances ab ON ab.asset_id = assets.id AND ab.account_id = $1
		WHERE assets.code = $2
		AND ab.asset_id IS NOT NULL
	`
	err = db.QueryRow(ctx, query, accountId, assetCode).Scan(&balance, &assetId)
	if err != nil {
		return decimal.Decimal{}, &assetId, err
	}
	return balance, &assetId, nil
}

func updateAccountBalance(ctx context.Context, db *pgxpool.Pool, id string, newBalance decimal.Decimal, assetId uuid.UUID) error {
	_, err := db.Exec(ctx, "UPDATE account_balances SET balance = $1 WHERE account_id = $2 AND asset_id = $3", newBalance, id, assetId)
	return err
}

func createAccountBalanceForAccount(ctx context.Context, db *pgxpool.Pool, accountId string, assetId uuid.UUID) error {
	_, err := db.Exec(ctx, "INSERT INTO account_balances (account_id, asset_id, balance) VALUES ($1, $2, $3)", accountId, assetId, decimal.NewFromInt(0))
	return err
}
