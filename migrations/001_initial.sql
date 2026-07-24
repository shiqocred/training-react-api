CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    password TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin', 'staff', 'customer')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS jwks (
    id TEXT PRIMARY KEY,
    kty VARCHAR(10) NOT NULL,
    crv VARCHAR(20) NOT NULL,
    x TEXT NOT NULL,
    secret_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expired_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS accounts (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL UNIQUE REFERENCES users(id),
    account_number TEXT NOT NULL UNIQUE,
    balance BIGINT NOT NULL DEFAULT 0 CHECK (balance >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS transactions (
    id TEXT PRIMARY KEY,
    actor_id TEXT NOT NULL REFERENCES users(id),
    customer_id TEXT NOT NULL REFERENCES users(id),
    source_account_id TEXT REFERENCES accounts(id),
    destination_account_id TEXT REFERENCES accounts(id),
    type TEXT NOT NULL CHECK (type IN ('deposit', 'withdraw', 'transfer')),
    direction TEXT NOT NULL CHECK (direction IN ('in', 'out')),
    amount BIGINT NOT NULL CHECK (amount > 0),
    balance_before BIGINT NOT NULL CHECK (balance_before >= 0),
    balance_after BIGINT NOT NULL CHECK (balance_after >= 0),
    reference_id TEXT,
    note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_transactions_customer_created ON transactions(customer_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_transactions_actor_created ON transactions(actor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_transactions_reference ON transactions(reference_id);

CREATE TABLE IF NOT EXISTS password_reset_otps (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    email TEXT NOT NULL,
    otp_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    verified_at TIMESTAMPTZ,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_password_reset_otps_user_created ON password_reset_otps(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS endpoint_logs (
    id TEXT PRIMARY KEY,
    user_id TEXT REFERENCES users(id) ON DELETE SET NULL ON UPDATE CASCADE,
    method TEXT NOT NULL,
    endpoint TEXT NOT NULL,
    request JSONB NOT NULL,
    response JSONB,
    status_code INTEGER,
    success BOOLEAN,
    error BOOLEAN,
    request_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    response_at TIMESTAMPTZ,
    duration_ms BIGINT
);

CREATE INDEX IF NOT EXISTS idx_endpoint_logs_request_at ON endpoint_logs(request_at DESC);
CREATE INDEX IF NOT EXISTS idx_endpoint_logs_endpoint_request_at ON endpoint_logs(endpoint, request_at DESC);
CREATE INDEX IF NOT EXISTS idx_endpoint_logs_user_request_at ON endpoint_logs(user_id, request_at DESC);

CREATE TABLE IF NOT EXISTS seed_runs (
    name TEXT PRIMARY KEY,
    ran_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
