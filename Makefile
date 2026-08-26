.PHONY: build check ci dev docs-check fmt fmt-check lint migrate run test test-integration vet

build:
	go build ./...

check: ci lint

ci: fmt-check vet test docs-check build

fmt:
	gofmt -w .

fmt-check:
	@files="$$(gofmt -l .)"; test -z "$$files" || { echo "The following files need formatting:"; echo "$$files"; echo "Run 'make fmt' and commit the result."; exit 1; }

lint:
	golangci-lint run

docs-check:
	go run github.com/getkin/kin-openapi/cmd/validate@v0.145.0 -- docs/openapi.yaml

migrate:
	@test -f .env || (echo ".env is required; copy .env.example to get started"; exit 1)
	@set -a; . ./.env; set +a; go run . migrate

run:
	@test -f .env || (echo ".env is required; copy .env.example to get started"; exit 1)
	@set -a; . ./.env; set +a; go run . serve

dev:
	@test -f .env || (echo ".env is required; copy .env.example to get started"; exit 1)
	@command -v air >/dev/null 2>&1 || (echo "Air is required; install it with: go install github.com/air-verse/air@latest"; exit 1)
	@exec air

test-integration:
	@test -n "$$TEST_DATABASE_URL" || (echo "TEST_DATABASE_URL is required"; exit 1)
	TEST_DATABASE_URL="$$TEST_DATABASE_URL" go test -p=1 -tags=integration -race ./internal/migrations ./internal/repositories

test:
	go test -race -cover ./...

vet:
	go vet ./...
