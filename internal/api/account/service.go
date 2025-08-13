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

func CreateNewAccountHandler(db *pgxpool.Pool) fiber.Handler {
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

		return c.Status(fiber.StatusCreated).JSON(CreateAccountResponseSchema{
			Id:       accountId.String(),
			Name:     account.Name,
			Balances: []AccountBalanceSchema{},
		})
	}
}

func GetAccountsHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Get pagination
		pagination := helper.GetPagination[AccountShowSchema](c)

		// Get total
		countQuery := "SELECT COUNT(*) FROM accounts"
		var total int
		if err := db.QueryRow(context.Background(), countQuery).Scan(&total); err != nil {
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

func GetAccountByIDHandler(db *pgxpool.Pool) fiber.Handler {
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

func UpdateAccountBalanceHandler(ctx context.Context, db *pgxpool.Pool, operation string) fiber.Handler {
	return func(c fiber.Ctx) error {
		id := c.Params("id")
		if id == "" {
			return fiber.ErrBadRequest
		}
		uuidId := uuid.MustParse(id)

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
		balance, assetId, err := GetAccountBalance(ctx, tx, uuidId, charge.AssetCode, nil)
		if err != nil {
			if err == pgx.ErrNoRows {
				CreateAccountBalanceForAccount(ctx, tx, uuidId, *assetId)
				balance = new(decimal.Decimal)
				*balance = decimal.NewFromInt(0)
			} else {
				return err
			}
		} else if balance == nil {
			CreateAccountBalanceForAccount(ctx, tx, uuidId, *assetId)
			balance = new(decimal.Decimal)
			*balance = decimal.NewFromInt(0)
		}

		switch operation {
		case "charge":
			*balance = balance.Add(*charge.Amount)
		case "remove":
			*balance = balance.Sub(*charge.Amount)
		}

		err = UpdateAccountBalance(ctx, tx, uuidId, *balance, *assetId)
		if err != nil {
			return err
		}

		// Commit transaction
		if err := tx.Commit(ctx); err != nil {
			return err
		}

		return c.JSON(UpdateBalanceResponseSchema{
			Balance:   balance,
			AssetCode: charge.AssetCode,
		})
	}
}

func GetAccountBalance(ctx context.Context, tx pgx.Tx, accountId uuid.UUID, assetCode *string, assetId *uuid.UUID) (*decimal.Decimal, *uuid.UUID, error) {
	if assetId == nil {
		err := tx.QueryRow(ctx, "SELECT id FROM assets WHERE code = $1", assetCode).Scan(&assetId)
		if err != nil {
			if err == pgx.ErrNoRows {
				return &decimal.Decimal{}, nil, fmt.Errorf("asset not found")
			}
			return &decimal.Decimal{}, nil, err
		}
	}

	var balance *decimal.Decimal
	query := `
		SELECT ab.balance
		FROM assets
		LEFT OUTER JOIN account_balances ab ON ab.asset_id = assets.id AND ab.account_id = $1
		WHERE assets.id = $2
	`
	err := tx.QueryRow(ctx, query, accountId, assetId).Scan(&balance)
	if err != nil {
		return &decimal.Decimal{}, assetId, err
	}
	return balance, assetId, nil
}

func UpdateAccountBalance(ctx context.Context, tx pgx.Tx, id uuid.UUID, newBalance decimal.Decimal, assetId uuid.UUID) error {
	_, err := tx.Exec(ctx, "UPDATE account_balances SET balance = $1 WHERE account_id = $2 AND asset_id = $3", newBalance, id, assetId)
	return err
}

func CreateAccountBalanceForAccount(ctx context.Context, tx pgx.Tx, accountId, assetId uuid.UUID) error {
	_, err := tx.Exec(ctx, "INSERT INTO account_balances (account_id, asset_id, balance) VALUES ($1, $2, 0)", accountId, assetId)
	return err
}
