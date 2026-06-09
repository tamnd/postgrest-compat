-- postgrest-compat shared test schema
-- Used by both docker/postgrest and docker/dbrest

create role anon nologin;
create role authenticated nologin;
grant usage on schema public to anon, authenticated;

-- countries / cities (for FK embed tests)
create table countries (
  id   serial primary key,
  name text   not null unique
);

create table cities (
  id         serial  primary key,
  name       text    not null,
  country_id integer references countries(id)
);

-- users
create table users (
  id        serial    primary key,
  username  text      not null unique,
  status    text      not null default 'ONLINE',
  age_range int4range,
  data      jsonb,
  arr       text[]
);

-- channels / messages (for relationship tests)
create table channels (
  id   serial primary key,
  slug text   not null unique
);

create table messages (
  id         serial  primary key,
  message    text,
  channel_id integer references channels(id),
  username   text    references users(username)
);

-- movies (for C# LINQ tests)
create table movies (
  id           uuid        primary key default gen_random_uuid(),
  name         text        not null,
  tags         text[]      not null default '{}',
  release_date date,
  watched_at   timestamptz
);

-- company + people (for Kotlin embed tests)
create table companies (
  id      serial primary key,
  name    text   not null,
  address text,
  phone   text
);

create table people (
  id         serial  primary key,
  name       text    not null,
  age        integer,
  company_id integer references companies(id)
);

-- grant
grant select, insert, update, delete on all tables in schema public to anon, authenticated;
grant usage, select on all sequences in schema public to anon, authenticated;

-- RPC functions
create or replace function get_status(name_param text)
returns text language sql security definer as $$
  select status from users where username = name_param limit 1;
$$;

create or replace function add(a integer, b integer)
returns integer language sql security definer as $$
  select a + b;
$$;

create or replace function get_user_count()
returns bigint language sql security definer as $$
  select count(*) from users;
$$;

grant execute on function get_status(text) to anon, authenticated;
grant execute on function add(integer, integer) to anon, authenticated;
grant execute on function get_user_count() to anon, authenticated;
