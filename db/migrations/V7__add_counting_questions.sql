-- V7: counting_questions — level-gated counting and arithmetic questions
CREATE TABLE counting_questions (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    level         VARCHAR(10)  NOT NULL CHECK (level IN ('easy','medium','hard')),
    question_json JSONB        NOT NULL,
    answer        INTEGER      NOT NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_counting_questions_level ON counting_questions(level);
