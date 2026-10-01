-- V9: badges + seed rows
CREATE TABLE badges (
    id        UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    feature   VARCHAR(20) NOT NULL CHECK (feature IN ('tracing','counting','global')),
    level     VARCHAR(20) NOT NULL CHECK (level IN ('beginner','explorer','master','all_rounder')),
    threshold INTEGER     NOT NULL,
    UNIQUE (feature, level)
);

-- Seed badge definitions
INSERT INTO badges (feature, level, threshold) VALUES
    ('tracing',  'beginner',    5),
    ('tracing',  'explorer',   15),
    ('tracing',  'master',     30),
    ('counting', 'beginner',    5),
    ('counting', 'explorer',   15),
    ('counting', 'master',     30),
    ('global',   'all_rounder', 1);  -- awarded when user holds ≥1 beginner in every feature
