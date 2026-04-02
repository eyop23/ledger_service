# Ledger Service

A double-entry bookkeeping API built in Go. Every transaction posts two or more entries (debits and credits) that must balance to zero — money is never created or destroyed, only moved.

---

## Running with Docker

### Prerequisites
- Docker and Docker Compose V2 installed (`docker compose` not `docker-compose`)

### Start the service

```bash
docker compose up --build
```

This will:
1. Start a PostgreSQL container
2. Run database migrations via goose
3. Start the API server on port `8084`

### Reset and restart with a clean database

```bash
docker compose down -v
docker compose up --build
```

---

## Running the tests

Integration tests use [testcontainers-go](https://golang.testcontainers.org/) — they spin up a real PostgreSQL container automatically. No manual setup needed.

```bash
go test ./tests/... -v
```

Each test gets its own isolated database and cleans up after itself.

---

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/accounts` | Create a new account |
| `GET` | `/accounts/{id}` | Get account details |
| `GET` | `/accounts/{id}/balance` | Get derived balance (sum of entries) |
| `GET` | `/accounts/{id}/entries` | List entries (cursor-paginated) |
| `POST` | `/transactions` | Post a double-entry transaction |
| `GET` | `/transactions/{id}` | Get a transaction and its entries |
| `GET` | `/audit` | Query the audit log (filterable) |
| `GET` | `/health` | Liveness and readiness check |

### Required headers

| Header | Description |
|--------|-------------|
| `X-Actor-ID` | Identity of the caller (stored in audit log) |
| `Content-Type: application/json` | Required for POST requests |

### Idempotency

Every `POST /transactions` requires an `idempotency_key`. Submitting the same key twice returns the original transaction with `HTTP 200` instead of `201` — no duplicate is created.

---

## Design Decisions

### Double-entry bookkeeping
Every transaction must have at least two entries where total debits equal total credits. Balances are never stored — they are derived at query time by summing all entries for an account (`SUM(CASE WHEN direction = 'CREDIT' THEN amount ELSE -amount END)`).

### Non-negative balance enforcement
Non-negative balance enforcement is **not enforced** — this is optional per the spec. Accounts can go negative. In a production system a dedicated bank/float account would be pre-funded and used as the source for all initial credits.

### Idempotency
The `idempotency_key` column has a `UNIQUE` constraint at the database level. If two concurrent requests arrive with the same key, PostgreSQL throws a `23505` unique violation. The service catches this and fetches and returns the original transaction — no application-level locking needed.

### REPEATABLE READ isolation
Transactions are posted inside a `REPEATABLE READ` database transaction. This prevents non-repeatable reads — data read at the start of the transaction remains consistent throughout. If PostgreSQL detects a serialization conflict it throws error `40001`, which the service catches and retries up to 3 times.

### Cursor-based pagination
Entry listing uses `(created_at, id)` as a composite cursor instead of page offsets. This avoids the problem of entries shifting between pages when new data is inserted. The cursor is base64-encoded and passed as a query parameter.

### Audit log completeness and audit-write trade-off

Every operation — including rejected transactions — writes an audit record.

**Decision:** the audit write always blocks the operation if it fails — there is no silent best-effort logging.

- **Posted transactions:** the audit write happens inside the same database transaction as the entries (`qtx`, which shares the open `tx`). If the audit write fails, `tx.Rollback` is called and both the transaction row and all entries are undone. Money never moves without a record.
- **Rejected transactions:** `rejectWithAudit` only writes one thing — the audit record. No transaction row or entries are inserted (the request was rejected before any money-related DB writes). If the audit write fails, the service returns `500` instead of the business error. The caller cannot tell the difference, but the rejection itself has no state to roll back.
- **Account creation:** the audit write is a separate DB call after the account insert. If it fails, the account already exists — no rollback. This is an acceptable trade-off since no money moves on account creation.

**Trade-off:** tying the audit write inside the DB transaction guarantees completeness at the cost of slightly higher latency. The alternative — async or best-effort audit — would be faster but risks losing records on crash, which is unacceptable for a financial ledger.

### Structured logging and metrics
All requests are logged as JSON using `slog` with request ID, actor, method, path, status, and latency. Prometheus metrics are exposed at `/metrics` — request counts by method/path/status and latency histograms.

---

## Project Structure

```
cmd/server/        — main.go: wires config, DB, services, handlers, router
config/            — environment-based configuration
internal/
  db/              — sqlc-generated database layer
  dto/             — request/response types and error codes
  handler/         — HTTP handlers (one file per resource)
  middleware/       — request ID, actor ID, logger, metrics
  pagination/      — cursor encode/decode
  service/         — business logic (account, transaction)
migrations/        — goose SQL migrations
tests/             — integration tests using testcontainers
```
