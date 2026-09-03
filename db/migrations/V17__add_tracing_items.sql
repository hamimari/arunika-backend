-- V5: tracing_items — stores guide paths for alphabet, number, and shape tracing
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE tracing_items (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    type         VARCHAR(20)  NOT NULL CHECK (type IN ('alphabet','number','shape')),
    label        VARCHAR(50)  NOT NULL,
    guide_path_json JSONB     NOT NULL,
    difficulty   SMALLINT     NOT NULL DEFAULT 1 CHECK (difficulty BETWEEN 1 AND 3),
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tracing_items_type ON tracing_items(type);
