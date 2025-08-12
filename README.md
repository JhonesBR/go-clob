# Central Limit Order Book (CLOB)
This project implements a simplified **Central Limit Order Book (CLOB)** and a matching engine to execute limit orders. It manages user accounts, balances, and order matching for a list of instruments (BTC/BRL). The implementation is written in Golang and uses PostgreSQL as the database, with Fiber as the web framework.

---

# Features

## Core features

1. Place Order
    - Allows users to place buy or sell orders for the an instrument.
    - Matches orders with existing ones in the order book if the price conditions are met.
    - Updates account balances accordingly.

2. Cancel Order
    - Allows users to cancel an order that is still open or partially filled.
    - Rolls back the reserved balance for the canceled order.

## Supporting features

1. Create account

2. List accounts
    - List accounts paginated
    - Get an account by ID

3. Add balance of an asset to an account

4. Remove balance of an asset of an account

5. List order book
    - List orders at order book paginated
    - Account and Instrument filter

---

# Technical Details

## Design Decisions

1. Database
    - PostgreSQL is used for its robust support for transactions and NUMERIC data type, which ensures precision for monetary values.
    - The pgxpool library is used for database connection pooling.

2. Framework:
    - Fiber is used for its simplicity and performance.

3. Monetary Values:
    - All monetary values are stored as NUMERIC in the database to handle cryptocurrency precision.

4. Simplifications:
    - N instruments are supported but `BTC/BRL` is already at the database init script
    - CRUD operations for assets and instruments are not implemented.

5. Transactions:
    - All operations involving balances and orders are wrapped in database transactions to ensure consistency and deal with race conditions

---

# API Endpoints

1. Create New Account
    - Endpoint: `POST /v1/accounts`
    - Description: Creates a new account.
    - Request Body:
    ```json
    {
        "name": "Account Name"
    }
    ```
    - Response:
    ```json
    {
        "id": "account-id",
        "name": "Account Name",
        "balances": []
    }
    ```

2. Get Accounts paginated
    - Endpoint: `GET /v1/accounts`
        - Query parameters:
            - page
            - size
    - Description: Retrieve accounts paginated with balance of assets
    - Response:
    ```json
    {
        "page": 1,
        "size": 50,
        "total": 1,
        "items": [
            {
                "id": "account-id",
                "name": "Account Name",
                "balances": [
                    {
                        "asset_id": "asset-id-1",
                        "balance": "X",
                        "asset_code": "BRL"
                    },
                    {
                        "asset_id": "asset-id-2",
                        "balance": "Y",
                        "asset_code": "BTC"
                    }
                ]
            }
        ]
    }
    ```

3. Get Account by ID
    - Endpoint: `GET /v1/accounts/:id`
    - Description: Retrieves an account by id with balance of assets.
    - Response:
    ```json
    {
        "id": "account-id",
        "name": "Account Name",
        "balances": [
            {
                "asset_id": "asset-id-1",
                "balance": "X",
                "asset_code": "BRL"
            },
            {
                "asset_id": "asset-id-2",
                "balance": "Y",
                "asset_code": "BTC"
            }
        ]
    }
    ```

4. **Place Order**
    - Endpoint: `POST /v1/order_book`
    - Description: Places a buy or sell order for the BTC/BRL instrument.
    - Request Body:
    ```json
    {
        "account_id": "account-id",
        "asset_code": "BTC",
        "quantity": "10",
        "price": "0.001",
        "order_type": "buy | sell"
    }
    ```
    - Response:
    `204 No Content`

5. **Cancel Order**
    - Endpoint: `POST /v1/order_book/:id/cancel`
    - Description: Cancels an open or partially filled order.
    - Response:
    `204 No Content`

6. Get Order Book
    - Endpoint: `GET /v1/order_book`
        - Query parameters:
            - page
            - size
            - account_id
            - instrument_id
    - Description: Retrieves the current state of the order book.
    - Response:
    ```json
    {
        "page": 1,
        "size": 50,
        "total": 1,
        "items": [
            {
                "id": "order-id",
                "account_id": "account-id",
                "instrument_id": "instrument-id",
                "type": "buy | sell",
                "status": "open | partially_filled | full_filled | canceled",
                "price": "0.001",
                "total_quantity": "10",
                "filled_quantity": "5"
            }
        ]
    }
    ```
---

# Steps to Run

1. Clone the Repository:

2. Install dependencies:
    ```bash
    go mod tidy
    ```

3. Start the Database:
    ```bash
    docker-compose up -d
    ```

4. Run the Application:
    ```bash
    go run cmd/main.go
    ```

---

# Database Schema

Tables

1. `accounts`
    - `id`: UUID (Primary Key)
    - `name`: String

2. `assets`
    - `id`: UUID (Primary Key)
    - `code`: String (e.g., "BTC", "BRL")
    - `name`: String

3. `account_balances`
    - `id`: UUID (Primary Key)
    - `account_id`: UUID (Foreign Key to accounts)
    - `asset_id`: UUID (Foreign Key to assets)
    - `balance`: NUMERIC

4. `instruments`
    - `id`: UUID (Primary Key)
    - `base_asset_id`: UUID (Foreign Key to assets)
    - `quote_asset_id`: UUID (Foreign Key to assets)

5. `order_book`
    - `id`: UUID (Primary Key)
    - `account_id`: UUID (Foreign Key to accounts)
    - `instrument_id`: UUID (Foreign Key to instruments)
    - `type`: String ("buy" or "sell")
    - `status`: String ("open", "partially_filled", "full_filled", "canceled")
    - `price`: NUMERIC
    - `total_quantity`: NUMERIC
    - `filled_quantity`: NUMERIC
    - `created_at`: TIMESTAMP

---

# Assumptions
    - Negative balances are allowed (no restrictions).
    - All operations are wrapped in transactions to ensure consistency and avoid race conditions.
    - Database is initialized with the `init.sql` script. The script creates the BTC/BRL instrument and assets.