# Step 1: Cache dependencies layer
FROM golang:1.24-alpine AS builder

WORKDIR /app

# COPY ONLY dependency definitions FIRST for optimal caching
COPY go.mod go.sum ./
RUN go mod download

# COPY source code AFTER dependencies are cached
COPY internal/ ./internal/
COPY apps/api/ ./apps/api/

# Build statically linked production binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/api ./apps/api/cmd/api

# Step 2: Minimal runtime image
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app

COPY --from=builder /bin/api /app/api

EXPOSE 8080
ENTRYPOINT ["/app/api"]
