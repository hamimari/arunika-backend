-- V11: user_subscriptions — one row per user, free by default
CREATE TABLE user_subscriptions (
    id                 UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id            UUID         NOT NULL UNIQUE,
    status             VARCHAR(10)  NOT NULL DEFAULT 'free' CHECK (status IN ('free','premium')),
    expires_at         TIMESTAMPTZ,
    midtrans_order_id  VARCHAR(100),
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Seed free subscription for all existing parents
INSERT INTO user_subscriptions (user_id)
SELECT id FROM parents
ON CONFLICT (user_id) DO NOTHING;
