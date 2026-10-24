-- Медиафайлы и их производные размеры.
CREATE TABLE media (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    filename    TEXT NOT NULL,
    path        TEXT NOT NULL UNIQUE,
    mime        TEXT NOT NULL,
    size        INTEGER NOT NULL,
    width       INTEGER NOT NULL,
    height      INTEGER NOT NULL,
    alt         TEXT NOT NULL DEFAULT '',
    uploaded_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    created_at  TEXT NOT NULL
);

CREATE TABLE media_variants (
    media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    name     TEXT NOT NULL,
    path     TEXT NOT NULL,
    width    INTEGER NOT NULL,
    height   INTEGER NOT NULL,
    PRIMARY KEY (media_id, name)
);
