# Build stage
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy go files
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o gateway main.go

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates wget

WORKDIR /app

# Copy binary only
COPY --from=builder /app/gateway .

# Non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
RUN chown appuser:appgroup /app/gateway
USER appuser

EXPOSE 8080

CMD ["./gateway"]