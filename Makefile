.PHONY: test test-integration run run-worker sync-contract capture-golden capture-schema tidy

test:
	go test ./...

test-integration:
	DATABASE_URL=postgres://postgres:password@localhost:5433/rv_marketplace_test go test -tags=integration ./...

run:
	go run ./cmd/server

run-worker:
	go run ./cmd/worker

sync-contract:
	@echo "Syncing OpenAPI, prompts, and regions from rv_marketplace..."
	@mkdir -p api prompts knowledge
	cp ../rv_marketplace/public/api-docs/v1/swagger.json api/openapi.yaml
	cp -r ../rv_marketplace/app/prompts/* prompts/
	cp ../rv_marketplace/app/knowledge/regions.yml knowledge/regions.yml

capture-golden:
	@echo "TODO: hit Rails API and refresh test/golden/*.golden.json"

# Refresh test/schema.sql from the Rails test DB. trekr_go never runs DDL;
# this dump only exists so CI can load the Rails schema into a fresh Postgres.
capture-schema:
	docker exec rv_marketplace-db-1 pg_dump -U postgres -d rv_marketplace_test \
		--schema-only --no-owner --no-privileges \
		| grep -vE '^\\(restrict|unrestrict)' > test/schema.sql

tidy:
	go mod tidy
