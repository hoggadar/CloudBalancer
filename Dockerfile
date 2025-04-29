FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o cloud_balancer ./cmd/api

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/cloud_balancer /app/cloud_balancer
COPY --from=builder /app/config/config.yaml /app/config/config.yaml

EXPOSE 8080

CMD ["/app/cloud_balancer"]