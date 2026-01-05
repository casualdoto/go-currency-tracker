# Build stage
FROM golang:1.21-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY ./shared/go.mod ./shared/
COPY ./rates-service/go.mod ./rates-service/go.sum ./rates-service/

# Download dependencies
RUN cd ./rates-service && go mod download

# Copy source code
COPY ./shared ./shared/
COPY ./rates-service ./rates-service/

# Build the application
RUN cd ./rates-service && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Set working directory
WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/rates-service/main .

# Expose port
EXPOSE 8081

# Command to run
CMD ["./main"]
