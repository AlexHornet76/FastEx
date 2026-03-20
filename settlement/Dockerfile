# Build stage
FROM golang:1.25.3-alpine AS builder

RUN apk add --no-cache ca-certificates wget

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o settlement main.go

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /app/settlement .

EXPOSE 8090
CMD ["./settlement"]