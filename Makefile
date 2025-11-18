# i used docker compose up --build and docker compose down -v for testing
# hope it will work with '-'
.PHONY: build run test clean docker-up docker-down

build:
	go build -o bin/pr-allocation-service ./cmd/pr-allocation-service

run:
	go run ./cmd/pr-allocation-service/main.go

test:
	go test -v -race -coverprofile=coverage.out ./...

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out


clean:
	rm -rf bin/
	rm -f coverage.out

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f

deps:
	go mod download
	go mod tidy

fmt:
	go fmt ./...

vet:
	go vet ./...


