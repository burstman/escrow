.PHONY: dev build templ css migrate fabric-up fabric-down chaincode-deploy test lint air

dev:
	air

css:
	npx tailwindcss -i web/static/css/input.css -o web/static/style.css

css-watch:
	npx tailwindcss -i web/static/css/input.css -o web/static/style.css --watch

air:
	air

build:
	templ generate && go build -o bin/escrow ./cmd/server

templ:
	templ generate

migrate:
	psql $$DATABASE_URL -f internal/db/migrations/001_init.sql

fabric-up:
	docker compose up -d peer0 orderer postgres

fabric-down:
	docker compose down

chaincode-deploy:
	cd chaincode/escrow && go build .

test:
	go test ./...

lint:
	golangci-lint run ./...
