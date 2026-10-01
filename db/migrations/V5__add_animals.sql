-- Animals table for the AR vocabulary collection
CREATE TABLE IF NOT EXISTS animals (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(255) NOT NULL,
    emoji      VARCHAR(20)  NOT NULL DEFAULT '',
    category   VARCHAR(50)  NOT NULL DEFAULT 'hutan', -- ternak | hutan | laut
    image_url  TEXT         NOT NULL DEFAULT '',
    bg_color   VARCHAR(20)  NOT NULL DEFAULT '#FFF3E0',
    fact       TEXT         NOT NULL DEFAULT '',
    is_unlocked BOOLEAN     NOT NULL DEFAULT false,
    created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    is_deleted BOOLEAN      NOT NULL DEFAULT false
);

CREATE INDEX IF NOT EXISTS idx_animals_category ON animals(category);
