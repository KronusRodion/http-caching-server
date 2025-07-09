FROM golang:1.24.3-alpine AS builder
RUN apk add --no-cache build-base
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o auth-app ./cmd/main.go

FROM alpine:latest
RUN apk add --no-cache tzdata
COPY --from=builder /app/auth-app /auth-app
COPY --from=builder /app/.env .
COPY internal/database/migrations /internal/database/migrations
COPY swagger ./swagger
EXPOSE 80

CMD ["/auth-app"]