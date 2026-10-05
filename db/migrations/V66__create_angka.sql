-- V66: Belajar Angka content (objects, numbers 1–20, levels with immutable
-- published versions), play sessions and per-child level progress, plus the
-- transparency flag object pictures are checked against.

ALTER TABLE content_assets ADD COLUMN has_alpha BOOLEAN;

-- Objects and numbers keep an editable draft and the published copy; levels
-- also keep every published version.
CREATE TABLE angka_objects (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    status     TEXT        NOT NULL DEFAULT 'draft'
                   CHECK (status IN ('draft', 'published', 'hidden')),
    draft      JSONB       NOT NULL,
    content    JSONB,
    draft_rev  INTEGER     NOT NULL DEFAULT 1,
    updated_by UUID REFERENCES admin_users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE angka_numbers (
    value      SMALLINT PRIMARY KEY CHECK (value BETWEEN 1 AND 20),
    is_free    BOOLEAN     NOT NULL DEFAULT FALSE,
    status     TEXT        NOT NULL DEFAULT 'draft'
                   CHECK (status IN ('draft', 'published', 'hidden')),
    draft      JSONB       NOT NULL,
    content    JSONB,
    draft_rev  INTEGER     NOT NULL DEFAULT 1,
    updated_by UUID REFERENCES admin_users(id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE angka_levels (
    id                   UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    sort_order           SMALLINT    NOT NULL,
    is_free              BOOLEAN     NOT NULL DEFAULT FALSE,
    status               TEXT        NOT NULL DEFAULT 'draft'
                             CHECK (status IN ('draft', 'published', 'hidden')),
    draft                JSONB       NOT NULL,
    draft_rev            INTEGER     NOT NULL DEFAULT 1,
    published_version_id UUID,
    updated_by           UUID REFERENCES admin_users(id),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- At most one free sample level.
CREATE UNIQUE INDEX one_free_angka_level ON angka_levels (is_free) WHERE is_free;

CREATE TABLE angka_level_versions (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    level_id     UUID        NOT NULL REFERENCES angka_levels(id),
    version      INTEGER     NOT NULL,
    content      JSONB       NOT NULL,
    published_by UUID        NOT NULL REFERENCES admin_users(id),
    published_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (level_id, version)
);

ALTER TABLE angka_levels
    ADD CONSTRAINT angka_levels_published_version_fk
        FOREIGN KEY (published_version_id) REFERENCES angka_level_versions(id);

-- A session pins the level version, the seed and the objects it started
-- with, so the server can regenerate exactly the questions the child saw.
CREATE TABLE angka_sessions (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    child_id         UUID        NOT NULL REFERENCES children(id) ON DELETE CASCADE,
    level_id         UUID        NOT NULL REFERENCES angka_levels(id),
    level_version_id UUID        NOT NULL REFERENCES angka_level_versions(id),
    seed             BIGINT      NOT NULL CHECK (seed BETWEEN 0 AND 4294967295),
    object_ids       JSONB       NOT NULL,
    answers          JSONB       NOT NULL DEFAULT '[]',
    status           TEXT        NOT NULL DEFAULT 'in_progress'
                         CHECK (status IN ('in_progress', 'completed', 'abandoned')),
    stars            SMALLINT CHECK (stars BETWEEN 1 AND 3),
    first_correct    SMALLINT,
    unlocked         JSONB,
    started_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at     TIMESTAMPTZ
);

CREATE INDEX idx_angka_sessions_child_level ON angka_sessions (child_id, level_id, status);

CREATE TABLE angka_level_progress (
    child_id           UUID        NOT NULL REFERENCES children(id) ON DELETE CASCADE,
    level_id           UUID        NOT NULL REFERENCES angka_levels(id),
    best_stars         SMALLINT    NOT NULL DEFAULT 0 CHECK (best_stars BETWEEN 0 AND 3),
    completed          BOOLEAN     NOT NULL DEFAULT FALSE,
    current_session_id UUID REFERENCES angka_sessions(id) ON DELETE SET NULL,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (child_id, level_id)
);

-- Numbers 1–20 as drafts with their names; 1–5 are the free sample.
INSERT INTO angka_numbers (value, is_free, draft)
SELECT n, n <= 5,
       jsonb_build_object('name', names[n], 'audio_asset_id', NULL, 'object_id', NULL)
FROM generate_series(1, 20) AS n,
     (SELECT ARRAY['satu', 'dua', 'tiga', 'empat', 'lima', 'enam', 'tujuh', 'delapan',
                   'sembilan', 'sepuluh', 'sebelas', 'dua belas', 'tiga belas',
                   'empat belas', 'lima belas', 'enam belas', 'tujuh belas',
                   'delapan belas', 'sembilan belas', 'dua puluh'] AS names) AS t;
