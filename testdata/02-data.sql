-- Seed data for the client compatibility tests.
-- Explicit IDs keep inserts idempotent across restarts.

insert into api.todos (id, done, task, due, tags) values
    (1, false, 'finish tutorial', '2030-01-01', '{go,sql}'),
    (2, true,  'pat the cat',     null,         '{pets}'),
    (3, false, 'do laundry',      '2030-06-15', '{chores,home}')
on conflict (id) do update set
    done = excluded.done, task = excluded.task,
    due = excluded.due,   tags = excluded.tags;

insert into api.persons (id, name, age, email) values
    (1, 'Alice', 30, 'alice@example.com'),
    (2, 'Bob',   25, 'bob@example.com'),
    (3, 'Carol', 35, 'carol@example.com')
on conflict (id) do nothing;

insert into api.assignments (id, person_id, todo_id) values
    (1, 1, 1),
    (2, 2, 3)
on conflict (id) do nothing;

insert into api.countries (id, name) values
    (1, 'Germany'),
    (2, 'France'),
    (3, 'Japan')
on conflict (id) do nothing;

insert into api.cities (id, name, country_id) values
    (1, 'Berlin', 1),
    (2, 'Munich', 1),
    (3, 'Paris',  2),
    (4, 'Tokyo',  3)
on conflict (id) do nothing;

insert into api.channels (id, slug) values
    (1, 'general'),
    (2, 'random'),
    (3, 'dev')
on conflict (id) do nothing;

insert into api.messages (id, message, channel_id, person_id) values
    (1, 'hello world',  1, 1),
    (2, 'how are you',  1, 2),
    (3, 'good morning', 2, 3),
    (4, 'deploy done',  3, 1)
on conflict (id) do nothing;

insert into api.companies (id, name, address, phone) values
    (1, 'Acme Corp', '123 Main St', '+1-555-0100'),
    (2, 'Globex',    '456 Oak Ave', '+1-555-0200')
on conflict (id) do nothing;

insert into api.people (id, name, age, company_id) values
    (1, 'John Doe',   30, 1),
    (2, 'Jane Smith', 25, 1),
    (3, 'Bob Jones',  40, 2)
on conflict (id) do nothing;

insert into api.movies (name, tags, release_date) values
    ('2001: A Space Odyssey', '{sci-fi,classic}',   '1968-04-02'),
    ('The Matrix',            '{sci-fi,action}',    '1999-03-31'),
    ('Mad Max: Fury Road',    '{action,adventure}', '2015-05-15'),
    ('Interstellar',          '{sci-fi,adventure}', '2014-11-05');

insert into private.items (id, name) values
    (1, 'item one'),
    (2, 'item two')
on conflict (id) do nothing;

-- reset sequences
select setval('api.todos_id_seq',       (select max(id) from api.todos));
select setval('api.persons_id_seq',     (select max(id) from api.persons));
select setval('api.assignments_id_seq', (select max(id) from api.assignments));
select setval('api.countries_id_seq',   (select max(id) from api.countries));
select setval('api.cities_id_seq',      (select max(id) from api.cities));
select setval('api.channels_id_seq',    (select max(id) from api.channels));
select setval('api.messages_id_seq',    (select max(id) from api.messages));
select setval('api.companies_id_seq',   (select max(id) from api.companies));
select setval('api.people_id_seq',      (select max(id) from api.people));
select setval('private.items_id_seq',   (select max(id) from private.items));
