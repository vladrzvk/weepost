.PHONY: run build test migrate-up migrate-down docker-up docker-down tidy

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

test:
	go test ./... -v

tidy:
	go mod tidy

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f api

migrate-up:
	@echo "Running migrations via app startup (auto-migrate on boot)"

lint:
	golangci-lint run ./...
