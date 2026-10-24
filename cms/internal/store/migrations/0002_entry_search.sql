-- Поисковая строка материала в нижнем регистре: lower() в SQLite понимает только ASCII,
-- поэтому строку для поиска по кириллице готовит приложение.
ALTER TABLE entries ADD COLUMN search TEXT NOT NULL DEFAULT '';
