include .env
export $(shell sed 's/=.*//' .env)

.PHONY: all build run test test-cover test-race docker-up docker-down migrate-up migrate-down seed

# Build
build:
	go build -o bin/api cmd/api/main.go

# Run
run:
	go run cmd/api/main.go

# Docker
docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f

# Database migrations
migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

# migrate-down:
# 	migrate -path migrations -database "$(DATABASE_URL)" down

migrate-create:
	migrate create -ext sql -dir migrations -seq $(name)

# Seed admin
seed:
	go run cmd/seed/main.go

# Testing
test:
	go test ./... -v

test-cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

test-race:
	go test ./... -race

test-leak:
	go test ./... -tags=leak

test-integration:
	go test ./tests/integration/... -v

# Swagger
swagger:
	swag init -g cmd/api/main.go -o docs

# Clean
clean:
	rm -rf bin/ coverage.out coverage.html

# Install tools
tools:
	go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	go install github.com/swaggo/swag/cmd/swag@latest

# All
all: docker-up migrate-up build
