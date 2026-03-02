FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/bambu-notifier ./cmd/bambu-notifier

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

COPY --from=builder /bin/bambu-notifier /bin/bambu-notifier

ENTRYPOINT ["/bin/bambu-notifier"]
CMD ["--config", "/config/config.toml"]
