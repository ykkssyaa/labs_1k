-- Просмотры материалов (сырые) и дневные свёртки.
CREATE TABLE page_views (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_id      INTEGER NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    visitor       TEXT NOT NULL,
    referrer_host TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL
);
CREATE INDEX page_views_dedupe ON page_views(entry_id, visitor, created_at);
CREATE INDEX page_views_time ON page_views(created_at);

-- Свёртка по материалу за день (день — по московскому времени, YYYY-MM-DD).
CREATE TABLE metrics_daily (
    day      TEXT NOT NULL,
    entry_id INTEGER NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    views    INTEGER NOT NULL,
    visitors INTEGER NOT NULL,
    PRIMARY KEY (day, entry_id)
);

-- Свёртка по сайту за день: уникальные посетители сайта не равны сумме по материалам.
CREATE TABLE metrics_site_daily (
    day      TEXT PRIMARY KEY,
    views    INTEGER NOT NULL,
    visitors INTEGER NOT NULL
);
