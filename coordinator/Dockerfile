# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY . .

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o backeye .

# Runtime stage
FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/backeye .

# The node binds to a random TCP port (tcp/0), so no fixed port to expose.
# If you pin a port later, add: EXPOSE <port>

ENTRYPOINT ["./backeye"]
