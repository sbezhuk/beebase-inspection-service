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
#  http://localhost:8081/.well-known/jwks.json and http://localhost:8083

# Option A: run Postgres in Docker, app on the host
docker compose up -d postgres
make migrate-up
make run

# Option B: run everything in Docker (migrations run once, automatically)
# requires BEEBASE_COMMON_GH_TOKEN - see "Building with beebase-common" below
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
  -d "{\"hive_id\":\"$HIVE_ID\",\"inspected_at\":\"2026-03-15\",\"notes\":\"queen seen, brood pattern good\"}"

curl "http://localhost:8080/api/v1/hives/$HIVE_ID/inspections" -H "Authorization: Bearer $TOKEN"
```

The full API surface is documented in [api/openapi.yaml](api/openapi.yaml).

Note: this repo's `docker-compose.yml` is for standalone single-service
development only. To run the full BeeBase stack together, use
`beebase-gateway`'s docker-compose, which builds every service from
sibling checkouts and routes between them.

### Building with beebase-common

This service depends on private BeeBase modules, including
`github.com/sbezhuk/beebase-common` and
`github.com/sbezhuk/beebase-health`. A local `go build`/`go test` resolves
them through your own git credentials. Building the Docker image
(`docker compose up --build` or a plain `docker build .`) needs a GitHub
PAT with `contents:read` on the required repositories, supplied as a
BuildKit secret so it never ends up in an image layer:

```bash
export BEEBASE_COMMON_GH_TOKEN=$(gh auth token)   # or any read-scoped PAT
export GOPRIVATE=github.com/sbezhuk/*
docker compose up --build
# or: docker build --secret id=github_token,env=BEEBASE_COMMON_GH_TOKEN .
```

CI needs the same token as a `BEEBASE_COMMON_GH_TOKEN` GitHub Actions
secret on this repo.

## Configuration

All configuration is via environment variables (see
[.env.example](.env.example) for the full list — it is a template only,
never read by the app, Docker Compose, or deployment tooling; copy it
once to create your real `.env`, which is what actually gets loaded).
Production configuration is generated at deploy time from AWS SSM
Parameter Store (see `beebase-gateway/deploy/deploy.sh`) — `.env.example`
is never used as a fallback, in development or in production.

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
| `REDIS_ADDR`                | *(required)*                 | Shared Redis session store for token revocation checks |
| `REDIS_CONNECT_TIMEOUT`     | `5s`                         | Timeout for the initial Redis connection   |
| `AUTH_JWKS_URL`             | *(required)*                 | auth-service's public key endpoint, used to verify access tokens |
| `INTERNAL_SERVICE_TOKEN`    | *(required)*                 | Credential for authenticated internal cleanup and existence calls |
| `HIVE_SERVICE_URL`          | *(required)*                 | hive-service's base URL, used to confirm hive ownership on create |
| `MEDIA_SERVICE_URL`         | *(required)*                 | media-service base URL used for media ownership and cascade cleanup |
| `PUBLIC_BASE_URL`           | *(required)*                 | Gateway base URL used to build media download URLs |
| `INSPECTION_WARNING_THRESHOLD_DAYS` | `14`                 | Days after a hive's latest inspection before it needs inspection - the single source of truth hive-service and statistics-service both read via `GET /api/v1/inspections/hive-status` |
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

Deletes are hard deletes. Hive deletion invokes this service's authenticated
internal cleanup endpoint before the parent hive is removed.

Normal hive/apiary deletion invokes this service's authenticated internal
cleanup endpoint. There is still no general event bus or outbox for
arbitrary out-of-band synchronization.

## Structured inspection assessments

Routine, QUEEN, BROOD, HEALTH, and FEEDING assessments are optional and stored as
nullable, typed columns on the inspection row, together with an assessment
version. BROOD stages and HEALTH sign lists use nullable PostgreSQL `TEXT[]`
columns: `NULL` means not assessed, while an empty array means assessed and no
values were observed. HEALTH values record beekeeper observations and concern;
they are not diagnoses or automatically calculated severity scores. FEEDING
keeps food stores (colony state) separate from feeding performed (beekeeper
action). SEASONAL explicitly records the beekeeper-selected phase, reuses
colony strength, and uses phase-specific store readiness rather than
reinterpreting absolute `FoodStores` values. Seasonal concerns preserve the
same nullable-versus-empty array semantics. This keeps
the existing inspection write atomic, makes the initial metrics queryable for
later analytics, and avoids the validation and interpretation costs of an
opaque JSON document. A missing metric is no observation; explicit enum values
such as `NOT_CHECKED` or `NOT_OBSERVED` are preserved. Version 1 is the only
accepted definition for each supported inspection type, and old rows retain
their stored version.

`InspectionType` is immutable after creation. PUT continues to accept the
legacy `type` field so the released client remains compatible; a changed type
is ignored, while an unchanged type succeeds.

## Development

```bash
make run                # go run ./cmd/server
make fmt                # go fmt ./...
make vet                # go vet ./...
make test               # unit tests: go test ./...
make lint               # golangci-lint run

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
