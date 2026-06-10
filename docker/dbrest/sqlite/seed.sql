-- Seed data for the client compatibility tests (SQLite version).
-- Uses 0/1 for booleans and JSON arrays for tags.
-- INSERT OR REPLACE keeps the seeding idempotent.

INSERT OR REPLACE INTO todos (id, done, task, due, tags) VALUES
    (1, 0, 'finish tutorial', '2030-01-01', '["go","sql"]'),
    (2, 1, 'pat the cat',     NULL,         '["pets"]'),
    (3, 0, 'do laundry',      '2030-06-15', '["chores","home"]');

INSERT OR REPLACE INTO persons (id, name, age, email) VALUES
    (1, 'Alice', 30, 'alice@example.com'),
    (2, 'Bob',   25, 'bob@example.com'),
    (3, 'Carol', 35, 'carol@example.com');

INSERT OR REPLACE INTO assignments (id, person_id, todo_id) VALUES
    (1, 1, 1),
    (2, 2, 3);

INSERT OR REPLACE INTO countries (id, name) VALUES
    (1, 'Germany'),
    (2, 'France'),
    (3, 'Japan');

INSERT OR REPLACE INTO cities (id, name, country_id) VALUES
    (1, 'Berlin', 1),
    (2, 'Munich', 1),
    (3, 'Paris',  2),
    (4, 'Tokyo',  3);

INSERT OR REPLACE INTO channels (id, slug) VALUES
    (1, 'general'),
    (2, 'random'),
    (3, 'dev');

INSERT OR REPLACE INTO messages (id, message, channel_id, person_id) VALUES
    (1, 'hello world',  1, 1),
    (2, 'how are you',  1, 2),
    (3, 'good morning', 2, 3),
    (4, 'deploy done',  3, 1);

INSERT OR REPLACE INTO companies (id, name, address, phone) VALUES
    (1, 'Acme Corp', '123 Main St', '+1-555-0100'),
    (2, 'Globex',    '456 Oak Ave', '+1-555-0200');

INSERT OR REPLACE INTO people (id, name, age, company_id) VALUES
    (1, 'John Doe',   30, 1),
    (2, 'Jane Smith', 25, 1),
    (3, 'Bob Jones',  40, 2);

INSERT OR IGNORE INTO movies (name, tags, release_date) VALUES
    ('2001: A Space Odyssey', '["sci-fi","classic"]',   '1968-04-02'),
    ('The Matrix',            '["sci-fi","action"]',    '1999-03-31'),
    ('Mad Max: Fury Road',    '["action","adventure"]', '2015-05-15'),
    ('Interstellar',          '["sci-fi","adventure"]', '2014-11-05');

INSERT OR REPLACE INTO items (id, name) VALUES
    (1, 'item one'),
    (2, 'item two');
