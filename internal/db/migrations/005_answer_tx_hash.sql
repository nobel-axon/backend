-- Add tx_hash column to app_answers for linking to block explorer
ALTER TABLE app_answers ADD COLUMN IF NOT EXISTS tx_hash TEXT;
