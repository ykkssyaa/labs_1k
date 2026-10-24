-- Таксономия: рубрики (иерархия) и теги, связи с материалами.
CREATE TABLE terms (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    vocabulary TEXT NOT NULL CHECK (vocabulary IN ('category', 'tag')),
    name       TEXT NOT NULL,
    slug       TEXT NOT NULL,
    parent_id  INTEGER REFERENCES terms(id) ON DELETE SET NULL,
    position   INTEGER NOT NULL DEFAULT 0,
    UNIQUE (vocabulary, slug)
);

CREATE TABLE entry_terms (
    entry_id INTEGER NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    term_id  INTEGER NOT NULL REFERENCES terms(id) ON DELETE CASCADE,
    PRIMARY KEY (entry_id, term_id)
);
CREATE INDEX entry_terms_term ON entry_terms(term_id);
