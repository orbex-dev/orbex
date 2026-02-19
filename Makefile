.PHONY: dev build migrate test clean

# Run in development mode
dev:
	@cp -n .env.example .env 2>/dev/null || true
	go run ./cmd/orbex-server

# Build the binary
build:
	go build -o dist/orbex-server ./cmd/orbex-server

# Start local Postgres
db-up:
	docker compose up -d

# Stop local Postgres
db-down:
	docker compose down

# Reset database (destroy and recreate)
db-reset:
	docker compose down -v
	docker compose up -d
	@echo "Waiting for Postgres to start..."
	@sleep 3
	@echo "Database reset complete. Run 'make dev' to apply migrations."

# Run tests
test:
	go test ./... -v -count=1

# Clean build artifacts
clean:
	rm -rf dist/ tmp/

# Format code
fmt:
	go fmt ./...

# Vet code
vet:
	go vet ./...
