CREATE EXTENSION "uuid-ossp";
GRANT SELECT ON ALL TABLES IN SCHEMA public TO root;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO root;

-- ------------------------------------------------------------------
-- Assets
CREATE TABLE IF NOT EXISTS assets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL
);

INSERT INTO assets (code, name) VALUES
    ('BTC', 'Bitcoin'),
    ('BRL', 'Brazilian Real');
-- ------------------------------------------------------------------


-- ------------------------------------------------------------------
-- Accounts
CREATE TABLE IF NOT EXISTS accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL
);
-- ------------------------------------------------------------------


-- ------------------------------------------------------------------
-- Account Balances
CREATE TABLE IF NOT EXISTS account_balances (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id),
    asset_id UUID NOT NULL REFERENCES assets(id),
    balance NUMERIC NOT NULL,
);
-- ------------------------------------------------------------------


-- ------------------------------------------------------------------
-- Instruments
CREATE TABLE IF NOT EXISTS instruments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    base_asset_id UUID NOT NULL REFERENCES assets(id),
    quote_asset_id UUID NOT NULL REFERENCES assets(id)
);

INSERT INTO instruments (base_asset_id, quote_asset_id) VALUES
    ((SELECT id FROM assets WHERE code = 'BTC'), (SELECT id FROM assets WHERE code = 'BRL'));
-- ------------------------------------------------------------------


-- ------------------------------------------------------------------
-- Order Book
CREATE TABLE IF NOT EXISTS order_book (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id),
    instrument_id UUID NOT NULL REFERENCES instruments(id),
    type TEXT NOT NULL CHECK (type IN ('buy', 'sell')),
    status TEXT NOT NULL CHECK (status IN ('open', 'partially_filled', 'full_filled', 'canceled')),
    price NUMERIC NOT NULL,
    total_quantity NUMERIC NOT NULL,
    filled_quantity NUMERIC NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
-- ------------------------------------------------------------------