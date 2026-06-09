-- Roles matching the dbrest conformance stack.
-- Both docker/postgrest and docker/dbrest load this file so they share the
-- same role hierarchy and the test harness can compare them on equal footing.

create role authenticator noinherit login password 'authenticator_pass';

create role web_anon nologin;
grant web_anon to authenticator;

create role web_user nologin;
grant web_user to authenticator;
