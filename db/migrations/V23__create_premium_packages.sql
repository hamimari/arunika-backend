-- Migration: Create premium_packages table
CREATE TABLE premium_packages (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name          VARCHAR(100) NOT NULL,
  subtitle      VARCHAR(255) NOT NULL,
  price_idr     INTEGER NOT NULL,
  type          VARCHAR(20) NOT NULL CHECK (type IN ('content', 'subscription')),
  badge_label   VARCHAR(50),
  is_best_value BOOLEAN NOT NULL DEFAULT FALSE,
  is_active     BOOLEAN NOT NULL DEFAULT TRUE,
  sort_order    INTEGER NOT NULL DEFAULT 0,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
