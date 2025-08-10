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

DELETE FROM assets;
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

DELETE FROM accounts;
-- ------------------------------------------------------------------


-- ------------------------------------------------------------------
-- Account Balances
CREATE TABLE IF NOT EXISTS account_balances (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id),
    asset_id UUID NOT NULL REFERENCES assets(id),
    balance NUMERIC NOT NULL
);
-- ------------------------------------------------------------------
