.PHONY: build run dev test lint migrate-up migrate-down sqlc-generate swagger docker-up docker-down clean

# Variables
BINARY=bin/server
DATABASE_URL ?= postgres://cenfit:cenfit@localhost:5432/cenfit_auth?sslmode=disable

## Build the binary
build:
	go build -ldflags="-s -w" -o $(BINARY) ./cmd/server

## Run the binary
run: build
	./$(BINARY)

## Run with air (hot-reload)
dev:
	air

## Run all tests
test:
	go test ./... -v -race -count=1

## Run tests with coverage
test-coverage:
	go test ./... -v -race -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html

## Run linter
lint:
	golangci-lint run

## Run database migrations up
migrate-up:
	migrate -path pkg/db/migrations -database "$(DATABASE_URL)" up

## Run database migrations down (1 step)
migrate-down:
	migrate -path pkg/db/migrations -database "$(DATABASE_URL)" down 1

## Generate sqlc code
sqlc-generate:
	sqlc generate

## Generate swagger docs
swagger:
	swag init -g cmd/server/main.go -o docs --parseDependency --parseInternal

## Start all services via Docker Compose
docker-up:
	docker compose up -d --build

## Start services in foreground; Ctrl+C stops and kills all containers
docker-watch:
	@trap 'docker compose down' INT; docker compose up

## Stop all services
docker-down:
	docker compose down

## Stop all services and remove volumes
docker-clean:
	docker compose down -v

## Tidy go modules
tidy:
	go mod tidy

## Clean build artifacts
clean:
	rm -rf bin/ tmp/ coverage.out coverage.html
