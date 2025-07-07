FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server

# Create a minimal image
FROM alpine:latest

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /server .

# Copy the web directory
COPY --from=builder /app/web ./web

# Copy the configs directory
COPY --from=builder /app/configs ./configs

# Create openapi directory and copy openapi.json
RUN mkdir -p /app/openapi
COPY --from=builder /app/openapi/openapi.json /app/openapi/

# Create a non-root user
RUN adduser -D -g '' appuser && \
    chown -R appuser:appuser /app

USER appuser

# Expose the port
EXPOSE 8081

# Run the application
CMD ["./server"] 