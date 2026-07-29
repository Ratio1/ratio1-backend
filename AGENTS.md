# Repository Guidelines

## Project Structure & Module Organization

This is a Go backend built with Gin and GORM. `cmd/` contains the CLI entry point. HTTP routing lives in `proxy/`, with endpoints in `proxy/handlers/` and authentication middleware in `proxy/middleware/`. Put business rules, scheduled jobs, and external-provider integrations in `service/`; keep PostgreSQL access in `storage/` and shared data structures in `model/`. Network-specific JSON configuration belongs in `config/`. Email and invoice assets live under `templates/html/`; operational helpers are in `scripts/` and `deployScripts/`. Tests are colocated with their packages as `*_test.go`.

## Build, Test, and Development Commands

- `go mod download` installs the module dependencies declared in `go.mod`.
- `go build -o ratio1-backend ./cmd` compiles the API executable.
- `EE_EVM_NET=devnet go run ./cmd --general-config ./config/` starts the API with `config/config.devnet.json`; required environment variables and PostgreSQL must be available.
- `go test ./crypto ./templates` runs focused, mostly self-contained tests.
- `go test ./...` runs the full suite. Some `service` and `storage` tests require a local `ratio1-db` PostgreSQL database or external services.
- `docker build --build-arg VERSION=dev -t ratio1-backend:dev .` reproduces the container build.

## Coding Style & Naming Conventions

Use standard Go formatting: tabs, `gofmt`, and grouped imports. Run `gofmt -w <changed-files>` and `go vet ./...` before review. Use PascalCase for exported identifiers, camelCase for internal names, and short lowercase package names. Preserve the existing dependency direction: handlers validate transport concerns, services implement behavior, and storers own persistence.

## Testing Guidelines

Use Go's `testing` package with `testify/require`, following existing names such as `TestFunction_ShouldOutcome`. Add unit tests beside changed code and prefer mocks for external APIs. There is no documented coverage threshold; focus on changed paths and failure cases. Identify integration tests explicitly and never run them against production data.

## Commit & Pull Request Guidelines

Recent history favors concise imperative subjects, often using `feat:`, `fix:`, or `chore:`. Keep commits scoped to one concern. Pull requests should explain behavior, configuration or schema impact, linked issues, and exact verification commands; include request/response examples when API contracts change.

## Security & Configuration

Copy configuration values locally but never commit `.env`, private keys, tokens, or customer data.

### Production Database Access

The local, ignored `.env.prod` contains a production administrator credential in this format:

```text
DATABASE_LINK=<host>:<port>:<database>:<user>:<password>
```

- Never print, log, paste, commit, or otherwise expose `DATABASE_LINK` or its password.
- Keep `.env.prod` ignored with permissions set to `600`.
- Never start the backend with `.env.prod` merely to test connectivity. Startup calls `storage.Connect()`, which runs GORM `AutoMigrate` and can modify the production schema.
- Default agent-initiated production sessions to one connection, a read-only transaction, SSL, and a short timeout.
- Avoid PII when aggregate or metadata queries are sufficient.
- Production writes, migrations, status resets, or corrections require explicit authorization for the exact operation, plus a backup, dry run, bounded transaction, and rollback plan.

The PostgreSQL client is installed through Homebrew's keg-only `libpq`. Do not `source` `.env.prod` or echo parsed values. From the repository root, use this read-only connectivity check:

```bash
PSQL_BIN="$(brew --prefix libpq)/bin/psql"
DATABASE_LINK_VALUE="$(sed -n 's/^DATABASE_LINK=//p' .env.prod)"
IFS=: read -r DATABASE_HOST_VALUE DATABASE_PORT_VALUE DATABASE_NAME_VALUE DATABASE_USER_VALUE DATABASE_PASSWORD_VALUE <<< "$DATABASE_LINK_VALUE"

PGPASSWORD="$DATABASE_PASSWORD_VALUE" \
PGOPTIONS='-c default_transaction_read_only=on -c statement_timeout=10000' \
"$PSQL_BIN" \
  --no-psqlrc \
  --set=ON_ERROR_STOP=1 \
  "host=$DATABASE_HOST_VALUE port=$DATABASE_PORT_VALUE dbname=$DATABASE_NAME_VALUE user=$DATABASE_USER_VALUE sslmode=require connect_timeout=10 application_name=codex_readonly_session" \
  --command="BEGIN READ ONLY; SELECT current_database(), current_user, current_setting('transaction_read_only'), current_setting('server_version'); ROLLBACK;"
```

Keep every approved inspection query between `BEGIN READ ONLY` and `ROLLBACK`. Do not remove `default_transaction_read_only`, the statement timeout, SSL, or `ON_ERROR_STOP`.
