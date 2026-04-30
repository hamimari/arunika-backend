-- V10: user_badges — earned badges per user (idempotent via unique constraint)
CREATE TABLE user_badges (
    id        UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id   UUID        NOT NULL,
    badge_id  UUID        NOT NULL REFERENCES badges(id) ON DELETE CASCADE,
    earned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, badge_id)
);

CREATE INDEX idx_user_badges_user ON user_badges(user_id);
