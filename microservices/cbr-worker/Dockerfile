# Build stage
FROM golang:1.21-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY ./shared/go.mod ./shared/
COPY ./cbr-worker/go.mod ./cbr-worker/go.sum ./cbr-worker/

# Download dependencies
RUN cd ./cbr-worker && go mod download

# Copy source code
COPY ./shared ./shared/
COPY ./cbr-worker ./cbr-worker/

# Build the application
RUN cd ./cbr-worker && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Set working directory
WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/cbr-worker/main .

# Command to run
CMD ["./main"]
