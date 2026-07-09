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

4. Start the frontend (proxies to `:3000`):

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

## Parity tracking

See [api/PARITY.md](api/PARITY.md) for route-by-route checklist.

Domain glossary lives in [rv_marketplace/CONTEXT.md](../rv_marketplace/CONTEXT.md).
