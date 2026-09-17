-- models.ArCards has always declared UpdatedAt, which GORM writes on every
-- create/update, but ar_cards (V1) was created without the column — so every
-- admin create/update of an AR card failed with SQLSTATE 42703.
ALTER TABLE ar_cards
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP;
