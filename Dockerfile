FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -o k8watch ./cmd/k8watch

# Final stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates sqlite-libs

WORKDIR /app

# Copy binary and web files
COPY --from=builder /app/k8watch .
COPY --from=builder /app/web ./web

# Create directory for database
RUN mkdir -p /data

EXPOSE 8080

ENTRYPOINT ["/app/k8watch"]
CMD ["--db", "/data/events.db", "--addr", ":8080"]
