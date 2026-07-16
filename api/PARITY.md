# trekr_go API parity checklist

Drop-in replacement for the Rails API in [rv_marketplace](https://github.com/rbowen/rv_marketplace).
Domain language: see `../rv_marketplace/CONTEXT.md`.

## Health

- [x] `GET /up` → 200

## Middleware (PR #1)

- [x] CORS headers match Rails (`config/initializers/cors.rb`)
- [x] Malformed JSON → 400

## Auth (PR #2–#5)

- [x] `POST /users` — registration
- [x] `POST /users/sign_in` — JWT in `Authorization` header (PR #3)
- [x] `DELETE /users/sign_out` (PR #4 — 204, tolerates malformed token)
- [x] Protected routes → 401 with JSend fail shape (PR #4 — `/api/v1` auth gate)
- [x] `POST /users/password` — reset token create (PR #5)
- [x] `PUT /users/password` — reset token update (PR #5)

## Listings (PR #6, #9)

- [x] `GET /api/v1/listings` (PR #6 — public, byte-parity vs Rails)
- [x] `GET /api/v1/listings/:id` (PR #6 — public, byte-parity vs Rails)
- [x] `GET /api/v1/listings/mine` (PR #9 — auth, owner-scoped)
- [x] `POST /api/v1/listings` (PR #9 — auth, region-on-save, ≥1 image, 422 msgs vs Rails)
- [x] `PUT`/`PATCH /api/v1/listings/:id` (PR #9 — owner-only, region re-resolve, keeps images)
- [x] `DELETE /api/v1/listings/:id` (PR #9 — owner-only, cascades dependents + purges images)

## Active Storage (PR #8, #10)

- [x] `GET /rails/active_storage/blobs/redirect/:signed_id/:filename` (PR #8)
- [x] `GET /rails/active_storage/disk/:encoded_key/:filename` — disk serve (PR #8)
- [x] Image write path — blob (28-char base36 key, MD5 checksum, `local` service) + attachment rows + disk write (PR #9)
- [x] `POST /api/v1/listings/:listing_id/images` — upload (PR #9, owner-only)
- [x] `DELETE /api/v1/listings/:listing_id/images/:id` — purge blob + attachment + file (PR #9)

## Bookings (PR #11)

- [x] `POST /api/v1/listings/:listing_id/bookings` (PR #11 — hirer-only, owner 403, overlap check under row lock)
- [x] `GET /api/v1/bookings` (PR #11 — hires+owns, newest first, participant shape)
- [x] `GET /api/v1/bookings/:id` (PR #11 — participants only, 404 non-participant, `trip_planning_available`)
- [x] `PATCH /api/v1/bookings/:id/confirm` (PR #11 — listing owner only)
- [x] `PATCH /api/v1/bookings/:id/reject` (PR #11 — listing owner only)

## Chats (PR #12)

- [ ] `GET /api/v1/chats`
- [ ] `POST /api/v1/chats`
- [ ] `GET /api/v1/chats/:id/messages`
- [ ] `POST /api/v1/chats/:id/messages`

## Region resolver (PR #7)

- [x] `knowledge/regions.yml` copied from Rails (`make sync-contract`)
- [x] `region.Resolve(town, state, postcode)` — mirrors `Region::Resolver.call` (exact-town match, file order)
- [x] `region.Find(slug)` / manifest unique-town invariant (ADR-0013), verified vs real Rails

## Search & AI (PR #13–#15)

- [x] Embedding worker (asynq) — PR #13: `Ai::Embedder`→Ollama, idempotent embed task (content_hash), enqueue on listing create/update, `ai_requests` logging (ADR-0011)
- [x] `POST /api/v1/listings/search` — PR #14: public NL search, pgvector cosine nearest-neighbours (limit 20), `score` per result, 422 blank / 503 embedder-down, `nl_search` ai_requests logging
- [x] `POST /api/v1/listings/generate_description` — PR #15: authed Claude call (anthropic-sdk-go), JSend success/fail/error mapping, `description_generator` ai_requests logging
- [x] `POST /api/v1/chats/:id/suggest_reply` — PR #15: owner-only (404→403→422 order), Claude reply draft, `chat_reply` ai_requests logging
- [x] AI rate limits + `ai_requests` logging (chat/description features) — PR #15: 10/hr per user, per feature; increments on admission (throttled never calls Claude); 429 JSend fail + `Retry-After`; Redis-backed (in-memory for tests)

## Out of scope

- Trip plan endpoints (not called by frontend)
