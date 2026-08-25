# beebase-inspection-service

Inspection tracking service for [BeeBase](https://github.com/sbezhuk/beebase-auth-service#trust-model),
an open-source backend for a beekeeper management application split into
microservices. See [CLAUDE.md](https://github.com/sbezhuk/beebase-auth-service/blob/main/CLAUDE.md)
for the architectural rules this service follows.

Register/login/refresh live in `beebase-auth-service`, apiaries in
`beebase-apiary-service`, hives in `beebase-hive-service` — this service
only manages inspections, and never trusts a user ID or a hive's
ownership from anywhere but a verified access token and a live check
against hive-service.

Related services: `beebase-auth-service` (users, refresh tokens, JWT
issuing), `beebase-apiary-service`, `beebase-hive-service`,
`beebase-gateway` (single entry point for clients).

## Requirements

- Go 1.27+
- PostgreSQL 16 (or Docker, to run it for you)
- [golang-migrate](https://github.com/golang-migrate/migrate) CLI, for applying
  migrations outside Docker: `make migrate-install`
- A running `beebase-auth-service` (or anything serving a compatible
  JWKS document) reachable at `AUTH_JWKS_URL`
- A running `beebase-hive-service` reachable at `HIVE_SERVICE_URL`

## Quick start

```bash
cp .env.example .env
# point AUTH_JWKS_URL and HIVE_SERVICE_URL at running services, e.g.
#   http://localhost:8081/.well-known/jwks.json and http://localhost:8083

# Option A: run Postgres in Docker, app on the host
docker compose up -d postgres
make migrate-up
make run

# Option B: run everything in Docker (migrations run once, automatically)
docker compose up --build
```

Verify it's up:

```bash
curl http://localhost:8080/health   # liveness — always 200 while the process is up
curl http://localhost:8080/ready    # readiness — 200 only if the database is reachable

TOKEN=...    # an access_token from auth-service's /api/v1/auth/register or /login
HIVE_ID=...  # a hive that TOKEN's owner created via hive-service

curl -X POST http://localhost:8080/api/v1/inspections \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d "{\"hive_id\":\"$HIVE_ID\",\"inspected_at\":\"2026-03-15T09:00:00Z\",\"notes\":\"queen seen, brood pattern good\"}"

curl "http://localhost:8080/api/v1/hives/$HIVE_ID/inspections" -H "Authorization: Bearer $TOKEN"
```

The full API surface is documented in [api/openapi.yaml](api/openapi.yaml).

Note: this repo's `docker-compose.yml` is for standalone single-service
development only. To run the full BeeBase stack together, use
`beebase-gateway`'s docker-compose, which builds every service from
sibling checkouts and routes between them.

## Configuration

All configuration is via environment variables (see
[.env.example](.env.example)):

| Variable                   | Default                    | Description                              |
| --------------------------- | --------------------------- | ----------------------------------------- |
| `APP_ENV`                  | `development`               | `development` or `production`             |
| `LOG_LEVEL`                 | `info`                       | `debug`, `info`, `warn`, `error`           |
| `HTTP_PORT`                 | `8080`                       | Port the HTTP server listens on           |
| `HTTP_READ_TIMEOUT`         | `5s`                         | Request read timeout                      |
| `HTTP_WRITE_TIMEOUT`        | `10s`                        | Response write timeout                    |
| `HTTP_IDLE_TIMEOUT`         | `60s`                        | Keep-alive idle timeout                   |
| `HTTP_SHUTDOWN_TIMEOUT`     | `15s`                        | Max time to wait for graceful shutdown    |
| `DATABASE_URL`              | *(required)*                 | PostgreSQL DSN                            |
| `DATABASE_CONNECT_TIMEOUT`  | `5s`                         | Timeout for the initial DB connection      |
| `AUTH_JWKS_URL`             | *(required)*                 | auth-service's public key endpoint, used to verify access tokens |
| `HIVE_SERVICE_URL`          | *(required)*                 | hive-service's base URL, used to confirm hive ownership on create |
| `TEST_DATABASE_URL`         | *(unset)*                    | Used only by `make test-integration`, never by the app |

## Project structure

```
cmd/server/                       entry point: wires config, logger, db, services, server
api/openapi.yaml                    API contract
migrations/                         SQL migrations (golang-migrate format)
internal/
  domain/inspection/                  Inspection entity, Repository port; no infrastructure dependency
  application/inspection/              use cases: create, get, list-for-a-hive, update, delete;
                                        HiveVerifier port (hive ownership check)
  platform/hiveclient/                HiveVerifier implemented by calling hive-service over HTTP
  repository/postgres/                domain port implemented against PostgreSQL (pgx, explicit SQL)
  transport/http/                    chi router, health/ready handlers
    inspection/                         inspection HTTP handlers, request validation, responses
```

logger, JSON response/error helpers, the graceful-shutdown server wrapper,
and JWKS-based access-token verification (`RequireAuth` middleware) all
come from [beebase-common](https://github.com/sbezhuk/beebase-common),
shared by every BeeBase service.

## Ownership

An inspection belongs to exactly one hive and, transitively, one apiary
and one user. Ownership is enforced in two layers:

1. **On create**, this service forwards the caller's own access token to
   hive-service's `GET /api/v1/hives/{hiveId}` and trusts its answer: a
   200 means whoever holds that token owns that hive, a 404 means they
   don't (or it doesn't exist) — collapsed into the same
   `404 hive_not_found` response either way, so a hive's existence can't
   be probed. Because hive-service's own ownership check is itself
   transitive against apiary-service, this single call confirms the
   *whole* chain (inspection → hive → apiary → user) without
   inspection-service ever calling apiary-service directly.
2. The verified owner's user ID is then denormalized onto the inspection
   row. Every later read/write (`GetByID`, `Update`, `Delete`) scopes its
   SQL by that `user_id` directly, exactly like hive-service and
   apiary-service scope their own resources — no cross-service call is
   needed after creation, since `hive_id` is immutable.

A request for another user's inspection returns the same
`404 inspection_not_found` as one that doesn't exist, never a `403`.
Listing inspections for a hive you don't own returns an empty list
(`200`), not an error, for the same reason: the caller simply has no
inspections there, which reveals nothing about the hive itself.

Deletes are soft (`deleted_at` is set, the row is retained) per the
project's offline-sync plan — inspections are a synchronizable entity.

**Known limitation:** if a hive (or its apiary) is deleted upstream, its
inspections here are not cascade-deleted or notified — there's no event
bus or outbox between services yet (CLAUDE.md defers full
synchronization). Those inspections become orphaned but remain
independently accessible to their owner until this is addressed.

## Development

```bash
make run               # go run ./cmd/server
make fmt                # go fmt ./...
make vet                # go vet ./...
make test               # unit tests: go test ./...
make lint                # golangci-lint run

make migrate-up         # apply migrations to DATABASE_URL
make migrate-down       # roll back the last migration
make migrate-new name=add_something   # scaffold a new migration pair

make build              # build binary into bin/
```

### Integration tests

Integration tests exercise the PostgreSQL repository and the full HTTP
CRUD flow — including a real JWKS round trip, a fake hive-service
standing in for the real cross-service ownership check, and two
independently authenticated users proving cross-user access is
impossible — against a real database. They're gated on
`TEST_DATABASE_URL` and skip themselves (not fail) if it's unset, and
every test runs inside a transaction that's rolled back afterward, so
they never leave rows behind or need manual cleanup.

```bash
docker compose up -d postgres
createdb -h localhost -p 5435 -U beebase beebase_inspection_test
migrate -path migrations -database "$TEST_DATABASE_URL" up

TEST_DATABASE_URL=postgres://beebase:beebase@localhost:5435/beebase_inspection_test?sslmode=disable \
  make test-integration
```
