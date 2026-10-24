-- Пользователи, сессии, реестр типов и материалы с ревизиями.
CREATE TABLE users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    email         TEXT NOT NULL UNIQUE,
    name          TEXT NOT NULL,
    role          TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    active        INTEGER NOT NULL DEFAULT 1,
    created_at    TEXT NOT NULL
);

CREATE TABLE sessions (
    token_hash TEXT PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    csrf       TEXT NOT NULL,
    expires_at TEXT NOT NULL
);
CREATE INDEX sessions_user ON sessions(user_id);

-- Версии схем типов материалов, загруженных из content_types/*.json.
CREATE TABLE content_types (
    name       TEXT PRIMARY KEY,
    version    INTEGER NOT NULL,
    schema     TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- Журнал применённых шагов миграций схем типов (не путать с db_migrations).
CREATE TABLE schema_migrations (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    type_name   TEXT NOT NULL,
    to_version  INTEGER NOT NULL,
    op          TEXT NOT NULL,
    affected    INTEGER NOT NULL,
    applied_at  TEXT NOT NULL
);

CREATE TABLE entries (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    type            TEXT NOT NULL,
    title           TEXT NOT NULL,
    slug            TEXT NOT NULL,
    status          TEXT NOT NULL CHECK (status IN ('draft','review','published','archived')),
    author_id       INTEGER NOT NULL REFERENCES users(id),
    seo_title       TEXT NOT NULL DEFAULT '',
    seo_description TEXT NOT NULL DEFAULT '',
    data            TEXT NOT NULL DEFAULT '{}',
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    published_at    TEXT,
    UNIQUE (type, slug)
);
CREATE INDEX entries_status ON entries(status, published_at);
CREATE INDEX entries_author ON entries(author_id);

CREATE TABLE entry_revisions (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_id   INTEGER NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    number     INTEGER NOT NULL,
    snapshot   TEXT NOT NULL,
    author_id  INTEGER NOT NULL,
    note       TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    UNIQUE (entry_id, number)
);
