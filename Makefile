.PHONY: test test-integration run run-worker sync-contract capture-golden tidy

test:
	go test ./...

test-integration:
	go test -tags=integration ./...

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

tidy:
	go mod tidy
