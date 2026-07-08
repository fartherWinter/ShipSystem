FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/shipsystem-api ./cmd/api

FROM alpine:3.22
WORKDIR /app
RUN addgroup -S shipsystem && adduser -S shipsystem -G shipsystem
COPY --from=builder /bin/shipsystem-api /usr/local/bin/shipsystem-api
USER shipsystem
EXPOSE 8080
CMD ["shipsystem-api"]
