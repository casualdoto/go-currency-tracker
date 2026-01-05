# Build stage
FROM golang:1.21-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY ./shared/go.mod ./shared/
COPY ./binance-worker/go.mod ./binance-worker/go.sum ./binance-worker/

# Download dependencies
RUN cd ./binance-worker && go mod download

# Copy source code
COPY ./shared ./shared/
COPY ./binance-worker ./binance-worker/

# Build the application
RUN cd ./binance-worker && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Set working directory
WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/binance-worker/main .

# Command to run
CMD ["./main"]
