# trekr_go

Go drop-in replacement for the [rv_marketplace](https://github.com/rbowen/rv_marketplace) Rails API.
Same Postgres database, same port 3000, unchanged React frontend.

## Prerequisites

- Go 1.22+
- Docker (for shared infrastructure in rv_marketplace)

## Dev workflow

1. Start shared services in rv_marketplace (db, redis, ollama only — not Rails):

   ```bash
   cd ../rv_marketplace
   docker compose up db redis ollama
   ```

2. Copy environment and align `SECRET_KEY_BASE` with Rails:

   ```bash
   cp .env.example .env
   ```

3. Run trekr_go (binds `:3000` — stop Rails first):

   ```bash
   make run
   ```

4. Run the background worker (processes embedding jobs; needs `redis` + `ollama`
   from step 1):

   ```bash
   make run-worker
   ```

   Listing create/update enqueues a semantic-search embedding job (ADR-0011) via
   asynq/Redis; the worker calls Ollama and writes `listing_embeddings`. The API
   process still serves requests without the worker running — embeddings just
   won't be generated until it is.

5. Start the frontend (proxies to `:3000`):

   ```bash
   cd ../rv_marketplace/frontend
   npm run dev
   ```

## Tests

```bash
make test                 # unit tests (no database required)
make test-integration     # HTTP + Postgres tests (requires rv_marketplace db on :5433)
```

Integration tests use `rv_marketplace_test` and skip automatically when the database is unavailable.
Start shared services first:

```bash
cd ../rv_marketplace
docker compose up db redis ollama
```

## Contract sync

```bash
make sync-contract   # copy openapi, prompts, regions.yml from rv_marketplace
```

## Timestamps & UTC (parity gotcha)

Rails stores every timestamp as UTC in `timestamp without time zone` columns and
its `as_json` renders them as UTC with a `Z` suffix and 3 fractional digits
(e.g. `2026-07-14T06:07:26.123Z`). To match this byte-for-byte, two rules hold:

- **Writes must be UTC.** GORM's default `NowFunc` returns `time.Now()` in the
  host's local zone, and pgx writes that local wall-clock straight into the
  `timestamp without time zone` column — so on a non-UTC host a Go-created row
  (e.g. `messages.created_at`, `chats.last_message_at`) would be stored and read
  back shifted by the host offset, diverging from Rails-created rows. `db.Open`
  therefore pins `NowFunc` to `time.Now().UTC()`. Any code that sets a timestamp
  by hand must likewise use `time.Now().UTC()` (see `ChatService.MarkRead`).
- **Rendering uses `formatRailsTime`** (`internal/httpapi/chat_serializer.go`),
  which calls `.UTC()` before formatting with the `Z07:00` layout. Serialize
  timestamps through it, not via ad-hoc `Format`, so the `Z`/precision stay
  consistent.

If you add a column or endpoint that renders a timestamp, keep both rules or the
value will silently drift from Rails on any developer machine not set to UTC.

## Parity tracking

See [api/PARITY.md](api/PARITY.md) for route-by-route checklist.

Domain glossary lives in [rv_marketplace/CONTEXT.md](../rv_marketplace/CONTEXT.md).
