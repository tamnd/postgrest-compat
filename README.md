# postgrest-compat

Compatibility test suite for PostgREST client libraries. Each test group covers one client library
and verifies that the exact HTTP traffic it generates produces correct responses from a live server.
Run the suite against PostgREST (the reference) or dbrest (the compatible implementation).

## What it tests

| Client | Language | Package | PR |
|--------|----------|---------|-----|
| postgrest-js | TypeScript/JS | `@supabase/postgrest-js` | #1 |
| postgrest-go | Go | `github.com/supabase-community/postgrest-go` | #2 |
| postgrest-py | Python | `supabase` (monorepo) | #3 |
| postgrest-rs | Rust | `postgrest` (crates.io) | #4 |
| postgrest-dart | Dart/Flutter | `postgrest` (pub.dev) | #5 |
| postgrest-kt | Kotlin | `io.supabase:postgrest-kt` | #6 |
| postgrest-csharp | C# | `Supabase.Postgrest` | #7 |
| postgrest-ex | Elixir | `supabase_postgrest` (Hex) | #8 |

Each test file sends the exact HTTP requests that the respective client library generates and checks
that the server returns the right status codes, headers, and JSON bodies.

## Running locally

### Against PostgREST (reference)

```bash
docker compose -f docker/postgrest/compose.yaml up -d
go test ./client/... -v
```

### Against dbrest

Build dbrest first from the `tamnd/dbrest` repo:

```bash
cd path/to/dbrest
docker build -t dbrest:local -f Dockerfile .
```

Then start the dbrest stack and run:

```bash
docker compose -f docker/dbrest/compose.yaml up -d
POSTGREST_URL=http://localhost:3001 go test ./client/... -v
```

### Run a single client group

```bash
go test ./client/js/...  -v
go test ./client/go/...  -v
go test ./client/py/...  -v
```

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTGREST_URL` | `http://localhost:3000` | Base URL of the server under test |
| `JWT_SECRET` | `reallyreallyreallyreallyverysafe` | JWT secret for auth tests |
| `ANON_ROLE` | `web_anon` | Anonymous role name |

## Schema

Both stacks load the SQL files from `testdata/` in order:

- `00-roles.sql` - authenticator, web\_anon, web\_user roles
- `01-schema.sql` - tables: todos, persons, assignments, channels, messages, countries, cities, movies, companies, people
- `02-data.sql` - seed rows
- `03-functions.sql` - RPC functions: get\_todos\_count, add\_todo, add, get\_person\_by\_name

## Project structure

```
client/
  js/      HTTP tests replicating @supabase/postgrest-js wire traffic
  go/      Tests using the actual postgrest-go library
  py/      HTTP tests replicating postgrest-py wire traffic
  rs/       HTTP tests replicating postgrest-rs wire traffic
  dart/    HTTP tests replicating postgrest-dart wire traffic
  kt/      HTTP tests replicating postgrest-kt wire traffic
  cs/      HTTP tests replicating Supabase.Postgrest wire traffic
  ex/      HTTP tests replicating supabase_postgrest wire traffic
docker/
  postgrest/  compose.yaml for PostgREST reference stack
  dbrest/     compose.yaml for dbrest stack
testdata/
  00-roles.sql ... 03-functions.sql
harness/
  harness.go  shared HTTP test helpers
```

## License

MIT
