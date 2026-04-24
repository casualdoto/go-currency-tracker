FROM golang:1.23-alpine AS builder
ENV GOPROXY=https://proxy.golang.org,direct
ENV CGO_ENABLED=0
WORKDIR /app

COPY shared/go.mod ./shared/
COPY normalization-service/go.mod normalization-service/go.sum ./normalization-service/
WORKDIR /app/normalization-service
RUN go mod download

WORKDIR /app
COPY shared/ ./shared/
COPY normalization-service/ ./normalization-service/
WORKDIR /app/normalization-service
RUN go build -o /normalization-service ./cmd/main.go

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /normalization-service .
CMD ["./normalization-service"]
