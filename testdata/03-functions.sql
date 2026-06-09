-- RPC functions for the client compatibility tests.

-- returns count of todos; used to test stable-function GET + POST
create or replace function api.get_todos_count()
returns integer language sql stable security invoker as $$
    select count(*)::integer from api.todos;
$$;
grant execute on function api.get_todos_count() to web_anon, web_user;

-- volatile insert; used to test volatile function + tx=rollback
create or replace function api.add_todo(task text)
returns api.todos language sql volatile security invoker as $$
    insert into api.todos (task) values (task) returning *;
$$;
grant execute on function api.add_todo(text) to web_user;

-- simple add: used by Rust and Kotlin tests
create or replace function api.add(a integer, b integer)
returns integer language sql stable security invoker as $$
    select a + b;
$$;
grant execute on function api.add(integer, integer) to web_anon, web_user;

-- returns a person by name; used by Go/JS RPC tests
create or replace function api.get_person_by_name(name_param text)
returns setof api.persons language sql stable security invoker as $$
    select * from api.persons where name = name_param;
$$;
grant execute on function api.get_person_by_name(text) to web_anon, web_user;

-- context readers: prove GUCs arrive correctly
create or replace function api.get_request_method()
returns text language sql stable security invoker as $$
    select current_setting('request.method', true);
$$;
grant execute on function api.get_request_method() to web_anon, web_user;

create or replace function api.get_jwt_claims()
returns json language sql stable security invoker as $$
    select coalesce(current_setting('request.jwt.claims', true), '{}')::json;
$$;
grant execute on function api.get_jwt_claims() to web_anon, web_user;

-- custom PT204 status
create or replace function api.raise_204()
returns void language plpgsql volatile security invoker as $$
begin
    raise sqlstate 'PT204';
end;
$$;
grant execute on function api.raise_204() to web_user;
