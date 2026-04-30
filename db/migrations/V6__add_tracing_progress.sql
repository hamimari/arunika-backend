-- V6: tracing_progress — per-attempt results submitted from the Flutter app
CREATE TABLE tracing_progress (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID         NOT NULL,
    child_id   UUID         NOT NULL,
    item_id    UUID         NOT NULL REFERENCES tracing_items(id) ON DELETE CASCADE,
    score      SMALLINT     NOT NULL CHECK (score BETWEEN 0 AND 100),
    passed     BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tracing_progress_user ON tracing_progress(user_id);
CREATE INDEX idx_tracing_progress_child ON tracing_progress(child_id);
