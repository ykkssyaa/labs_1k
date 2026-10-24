-- Журнал действий пользователей. user_name копируется, чтобы запись пережила удаление пользователя.
CREATE TABLE audit_log (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER,
    user_name  TEXT NOT NULL,
    action     TEXT NOT NULL,
    object     TEXT NOT NULL DEFAULT '',
    object_id  INTEGER NOT NULL DEFAULT 0,
    details    TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);
CREATE INDEX audit_log_user ON audit_log(user_id, created_at);
CREATE INDEX audit_log_action ON audit_log(action, created_at);
