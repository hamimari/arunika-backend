-- V64: Belajar Huruf content (drafts + immutable published versions) and
-- per-child progress.

CREATE TABLE letters (
    id                   UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    upper                CHAR(1)     NOT NULL UNIQUE CHECK (upper BETWEEN 'A' AND 'Z'),
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

-- At most one free sample letter.
CREATE UNIQUE INDEX one_free_letter ON letters (is_free) WHERE is_free;

CREATE TABLE letter_versions (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    letter_id    UUID        NOT NULL REFERENCES letters(id),
    version      INTEGER     NOT NULL,
    content      JSONB       NOT NULL,
    published_by UUID        NOT NULL REFERENCES admin_users(id),
    published_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (letter_id, version)
);

ALTER TABLE letters
    ADD CONSTRAINT letters_published_version_fk
        FOREIGN KEY (published_version_id) REFERENCES letter_versions(id);

CREATE TABLE letter_progress (
    child_id      UUID         NOT NULL REFERENCES children(id) ON DELETE CASCADE,
    letter_id     UUID         NOT NULL REFERENCES letters(id),
    kenali_done   BOOLEAN      NOT NULL DEFAULT FALSE,
    dengar_done   BOOLEAN      NOT NULL DEFAULT FALSE,
    tebalkan_done BOOLEAN      NOT NULL DEFAULT FALSE,
    best_score    NUMERIC(3,2) CHECK (best_score BETWEEN 0 AND 1),
    attempts      INTEGER      NOT NULL DEFAULT 0,
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (child_id, letter_id)
);

-- A–Z as drafts; A is the free sample.
INSERT INTO letters (upper, sort_order, is_free, draft)
SELECT chr(64 + n), n, n = 1,
       jsonb_build_object(
           'upper', chr(64 + n),
           'lower', lower(chr(64 + n)),
           'kenali', jsonb_build_object('word', '', 'highlight', '[]'::jsonb, 'image_asset_id', NULL),
           'dengar', jsonb_build_object('letter_audio_id', NULL, 'word_audio_id', NULL),
           'tebalkan', jsonb_build_object('grid', 300, 'lower_required', FALSE,
                                          'upper', '[]'::jsonb, 'lower', '[]'::jsonb),
           'feedback', jsonb_build_object(
               'success', 'Keren! Huruf ' || chr(64 + n) || ' rapi!',
               'retry', 'Belum pas, ayo lagi!',
               'hint', 'Mulai dari angka 1, lalu ikuti titik-titik sampai ujung.'))
FROM generate_series(1, 26) AS n;
