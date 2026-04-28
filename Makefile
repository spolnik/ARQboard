.PHONY: dev test lint generate migrate-up docker-build

DATABASE_URL ?= sqlite://data/arqboard.db
APP_URL ?= http://localhost:8080
SESSION_SECRET ?= development-session-secret-change-me

dev:
	cd web && npm run build
	DATABASE_URL="$(DATABASE_URL)" APP_URL="$(APP_URL)" SESSION_SECRET="$(SESSION_SECRET)" go run ./cmd/arqboard migrate
	DATABASE_URL="$(DATABASE_URL)" APP_URL="$(APP_URL)" SESSION_SECRET="$(SESSION_SECRET)" go run ./cmd/arqboard serve

test:
	go test ./...
	cd web && npm test

lint:
	go test ./...
	cd web && npm run lint

generate:
	sqlc generate

migrate-up:
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/arqboard migrate

migrate-up-postgres:
	DATABASE_URL="postgres://arqboard:arqboard@localhost:5432/arqboard?sslmode=disable" go run ./cmd/arqboard migrate

docker-build:
	docker build -t arqboard:dev .
