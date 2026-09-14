-- Backs the Flutter app's watch-history/progress feature (dongeng_history_api.dart),
-- which already calls POST/PUT /fairy-tales/:id/play and GET /fairy-tales/history —
-- those routes never existed server-side, so Gin's GET /fairy-tales/:id wildcard
-- swallowed "history" as an id and crashed on the UUID parse.
CREATE TABLE dongeng_play_history (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id          UUID NOT NULL REFERENCES parents(id) ON DELETE CASCADE,
  dongeng_id       UUID NOT NULL REFERENCES dongengs(id) ON DELETE CASCADE,
  progress_seconds INTEGER NOT NULL DEFAULT 0,
  play_count       INTEGER NOT NULL DEFAULT 0,
  started_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, dongeng_id)
);

CREATE INDEX idx_dongeng_play_history_user_id ON dongeng_play_history(user_id);
