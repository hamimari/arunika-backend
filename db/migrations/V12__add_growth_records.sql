-- V12: growth_records — child weight/height history logged by parents
CREATE TABLE growth_records (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    child_id    UUID            NOT NULL,
    recorded_at DATE            NOT NULL,
    weight_kg   NUMERIC(5,2)    NOT NULL CHECK (weight_kg > 0),
    height_cm   NUMERIC(5,2)    NOT NULL CHECK (height_cm > 0),
    created_at  TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_growth_records_child ON growth_records(child_id, recorded_at);
