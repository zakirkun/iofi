.PHONY: dev web build run up down test tidy

# Frontend dev server proxies /api to the Go process on :8080.
dev:
	cd web && npm run dev

web:
	cd web && npm install && npm run build

build: web
	go build -o bin/iofi ./cmd/iofi

run:
	go run ./cmd/iofi

up:
	docker compose up -d

down:
	docker compose down

test:
	go test ./...

tidy:
	go mod tidy && gofmt -l -w .
