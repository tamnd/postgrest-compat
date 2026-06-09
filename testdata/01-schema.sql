-- Schema for the client compatibility test suite.
-- Extends the base dbrest conformance tables (todos/persons/assignments) with
-- the richer tables that the eight client library test groups need.

create schema if not exists api;

-- base tables from dbrest conformance suite

create table api.todos (
    id   serial  primary key,
    done boolean not null default false,
    task text    not null,
    due  date,
    tags text[]  not null default '{}'
);

create table api.persons (
    id    serial primary key,
    name  text   not null,
    age   int,
    email text   unique
);

create table api.assignments (
    id        serial primary key,
    person_id int    references api.persons(id) on delete cascade,
    todo_id   int    references api.todos(id)   on delete cascade,
    unique (person_id, todo_id)
);

-- channels + messages for JS / Go / Python FK-embed tests

create table api.channels (
    id   serial primary key,
    slug text   not null unique
);

create table api.messages (
    id         serial  primary key,
    message    text,
    channel_id integer references api.channels(id),
    person_id  integer references api.persons(id)
);

-- countries + cities for JS embed tests

create table api.countries (
    id   serial primary key,
    name text   not null unique
);

create table api.cities (
    id         serial  primary key,
    name       text    not null,
    country_id integer references api.countries(id)
);

-- movies for C# LINQ tests

create table api.movies (
    id           uuid        primary key default gen_random_uuid(),
    name         text        not null,
    tags         text[]      not null default '{}',
    release_date date,
    watched_at   timestamptz
);

-- companies + people for Kotlin embedded-resource tests

create table api.companies (
    id      serial primary key,
    name    text   not null,
    address text,
    phone   text
);

create table api.people (
    id         serial  primary key,
    name       text    not null,
    age        integer,
    company_id integer references api.companies(id)
);

-- items in private schema for schema-switching tests (Accept-Profile / Content-Profile)

create schema if not exists private;

create table private.items (
    id   serial primary key,
    name text   not null
);

-- permissions: web_anon can read and write everything for compat tests
-- (production would keep anon read-only; we open writes so both servers
-- return identical 201/204 on write test cases without a JWT secret)

grant usage on schema api to web_anon, web_user;
grant select, insert, update, delete on all tables in schema api to web_anon, web_user;
grant usage, select on all sequences in schema api to web_anon, web_user;

grant usage  on schema private to web_anon, web_user;
grant select on private.items  to web_anon, web_user;
grant usage, select on all sequences in schema private to web_anon, web_user;
