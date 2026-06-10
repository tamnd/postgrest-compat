-- SQLite schema for the client compatibility test suite.
-- All tables live in one database file; no role/grant statements (NativeRoles=false).

CREATE TABLE IF NOT EXISTS todos (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    done BOOLEAN NOT NULL DEFAULT 0,
    task TEXT    NOT NULL UNIQUE,
    due  TEXT,
    tags JSON    NOT NULL DEFAULT '[]'
);

CREATE TABLE IF NOT EXISTS persons (
    id    INTEGER PRIMARY KEY AUTOINCREMENT,
    name  TEXT    NOT NULL,
    age   INTEGER,
    email TEXT    UNIQUE
);

CREATE TABLE IF NOT EXISTS assignments (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    person_id INTEGER REFERENCES persons(id) ON DELETE CASCADE,
    todo_id   INTEGER REFERENCES todos(id)   ON DELETE CASCADE,
    UNIQUE (person_id, todo_id)
);

CREATE TABLE IF NOT EXISTS channels (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT    NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS messages (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    message    TEXT,
    channel_id INTEGER REFERENCES channels(id),
    person_id  INTEGER REFERENCES persons(id)
);

CREATE TABLE IF NOT EXISTS countries (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT    NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS cities (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL,
    country_id INTEGER REFERENCES countries(id)
);

CREATE TABLE IF NOT EXISTS movies (
    id           TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    name         TEXT NOT NULL,
    tags         JSON NOT NULL DEFAULT '[]',
    release_date TEXT,
    watched_at   TEXT
);

CREATE TABLE IF NOT EXISTS companies (
    id      INTEGER PRIMARY KEY AUTOINCREMENT,
    name    TEXT    NOT NULL,
    address TEXT,
    phone   TEXT
);

CREATE TABLE IF NOT EXISTS people (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL,
    age        INTEGER,
    company_id INTEGER REFERENCES companies(id)
);

-- private-schema equivalent: items is accessible via Accept-Profile: private
CREATE TABLE IF NOT EXISTS items (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT    NOT NULL
);
