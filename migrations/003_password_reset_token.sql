ALTER TABLE password_reset_otps
ADD COLUMN IF NOT EXISTS reset_token_hash TEXT,
ADD COLUMN IF NOT EXISTS reset_token_expires_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_password_reset_otps_email_reset ON password_reset_otps(email, created_at DESC);
