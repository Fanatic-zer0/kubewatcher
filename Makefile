.PHONY: build run clean docker docker-run test

# Build the application
build:
	go build -o k8watch ./cmd/k8watch

# Run locally
run:
	go run ./cmd/k8watch

# Clean build artifacts
clean:
	rm -f k8watch
	rm -rf data/
	rm -f events.db

# Build Docker image
docker:
	docker build -t k8watch:latest .

# Run with Docker Compose
docker-run:
	docker-compose up -d

# Stop Docker Compose
docker-stop:
	docker-compose down

# View Docker logs
docker-logs:
	docker-compose logs -f

# Run tests
test:
	go test ./...

# Install dependencies
deps:
	go mod download
	go mod tidy

# Quick start (build and run)
start: build
	./k8watch

# Development mode with auto-reload (requires air: go install github.com/cosmtrek/air@latest)
dev:
	air
