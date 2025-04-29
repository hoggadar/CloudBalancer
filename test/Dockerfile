FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY main.go .

# Build the test server
RUN go build -o echo_server main.go

# Final lightweight image
FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/echo_server /app/echo_server

EXPOSE 8080

# Set default environment variables
ENV SERVER_PORT=8080 \
    SERVER_MESSAGE="Hello from Docker test server"

CMD ["/app/echo_server"] 