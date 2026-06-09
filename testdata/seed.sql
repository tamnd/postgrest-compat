-- Seed data for postgrest-compat tests

insert into countries (name) values
  ('Germany'),
  ('France'),
  ('Japan');

insert into cities (name, country_id) values
  ('Berlin',   1),
  ('Munich',   1),
  ('Paris',    2),
  ('Lyon',     2),
  ('Tokyo',    3),
  ('Osaka',    3);

insert into companies (name, address, phone) values
  ('Acme Corp', '123 Main St', '+1-555-0100'),
  ('Globex',    '456 Oak Ave', '+1-555-0200');

insert into users (username, status, age_range, data, arr) values
  ('alice',   'ONLINE',  '[20,30)',  '{"role":"admin"}',  '{go,rust}'),
  ('bob',     'ONLINE',  '[30,40)',  '{"role":"user"}',   '{go,python}'),
  ('charlie', 'OFFLINE', '[40,50)',  '{"role":"user"}',   '{java}'),
  ('diana',   'OFFLINE', '[25,35)',  '{"role":"admin"}',  '{rust,c}'),
  ('eve',     'ONLINE',  '[18,25)',  null,                null);

insert into channels (slug) values
  ('general'),
  ('random'),
  ('dev');

insert into messages (message, channel_id, username) values
  ('hello world',     1, 'alice'),
  ('how are you',     1, 'bob'),
  ('good morning',    2, 'charlie'),
  ('random thought',  2, 'diana'),
  ('deploy done',     3, 'alice'),
  ('PR merged',       3, 'bob');

insert into people (name, age, company_id) values
  ('John Doe',   30, 1),
  ('Jane Smith', 25, 1),
  ('Bob Jones',  40, 2);

insert into movies (name, tags, release_date) values
  ('2001: A Space Odyssey', '{sci-fi,classic}',    '1968-04-02'),
  ('The Matrix',            '{sci-fi,action}',     '1999-03-31'),
  ('Mad Max: Fury Road',    '{action,adventure}',  '2015-05-15'),
  ('Interstellar',          '{sci-fi,adventure}',  '2014-11-05');
