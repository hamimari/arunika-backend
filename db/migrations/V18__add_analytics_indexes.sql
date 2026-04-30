-- V18: indexes for analytics and user_subscriptions payment queries
-- Supports payment analytics queries grouped by status and date range
CREATE INDEX IF NOT EXISTS idx_user_subscriptions_created_at ON user_subscriptions(created_at);
CREATE INDEX IF NOT EXISTS idx_user_subscriptions_status      ON user_subscriptions(status);

-- Composite index for payment analytics (status + date range)
CREATE INDEX IF NOT EXISTS idx_user_subscriptions_status_created_at ON user_subscriptions(status, created_at);
