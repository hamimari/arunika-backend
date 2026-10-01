-- V8: counting_progress — per-answer results
CREATE TABLE counting_progress (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID         NOT NULL,
    child_id    UUID         NOT NULL,
    question_id UUID         NOT NULL REFERENCES counting_questions(id) ON DELETE CASCADE,
    is_correct  BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_counting_progress_user ON counting_progress(user_id);
CREATE INDEX idx_counting_progress_child ON counting_progress(child_id);
