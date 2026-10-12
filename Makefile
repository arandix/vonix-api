BINARY=vonix-api
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: build run test lint fmt tidy docker-build

build:
	go build -ldflags="-X main.version=$(VERSION)" -o bin/$(BINARY) ./cmd/server

run:
	ENV=development go run ./cmd/server

test:
	go test -race -cover ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .
	goimports -w .

tidy:
	go mod tidy

docker-build:
	docker build -t vonix-api:$(VERSION) .

docker-run:
	docker run --rm -p 8080:8080 --env-file .env vonix-api:$(VERSION)

.env:
	cp .env.example .env
