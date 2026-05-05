
# Development stage
FROM golang:1.25-alpine AS dev

WORKDIR /app

RUN apk add --no-cache git ca-certificates tzdata

RUN go install github.com/air-verse/air@latest

COPY go.mod go.sum ./
RUN go mod download

COPY . .

CMD ["air", "-c", ".air.toml"]


# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/server ./cmd/server


# Production stage
FROM alpine:3.22 AS prod

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /out/server /usr/local/bin/server

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/server"]