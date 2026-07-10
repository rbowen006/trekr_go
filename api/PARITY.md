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

- [ ] `GET /api/v1/listings`
- [ ] `GET /api/v1/listings/:id`
- [ ] `GET /api/v1/listings/mine`
- [ ] `POST /api/v1/listings`
- [ ] `PATCH /api/v1/listings/:id`
- [ ] `DELETE /api/v1/listings/:id`

## Active Storage (PR #8, #10)

- [x] `GET /rails/active_storage/blobs/redirect/:signed_id/:filename` (PR #8)
- [x] `GET /rails/active_storage/disk/:encoded_key/:filename` — disk serve (PR #8)
- [ ] Image upload on listing create/update
- [ ] Image delete

## Bookings (PR #11)

- [ ] `POST /api/v1/bookings`
- [ ] `GET /api/v1/bookings`
- [ ] `GET /api/v1/bookings/:id`
- [ ] `PATCH /api/v1/bookings/:id/confirm`
- [ ] `PATCH /api/v1/bookings/:id/reject`

## Chats (PR #12)

- [ ] `GET /api/v1/chats`
- [ ] `POST /api/v1/chats`
- [ ] `GET /api/v1/chats/:id/messages`
- [ ] `POST /api/v1/chats/:id/messages`

## Search & AI (PR #13–#15)

- [ ] Embedding worker (asynq)
- [ ] `POST /api/v1/listings/search`
- [ ] `POST /api/v1/listings/generate_description`
- [ ] `POST /api/v1/chats/:id/suggest_reply`
- [ ] AI rate limits + `ai_requests` logging

## Out of scope

- Trip plan endpoints (not called by frontend)
