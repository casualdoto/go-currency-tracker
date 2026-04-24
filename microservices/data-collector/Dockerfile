FROM golang:1.23-alpine AS builder
ENV GOPROXY=https://proxy.golang.org,direct
ENV CGO_ENABLED=0
WORKDIR /app

COPY shared/go.mod ./shared/
COPY data-collector/go.mod data-collector/go.sum ./data-collector/
WORKDIR /app/data-collector
RUN go mod download

WORKDIR /app
COPY shared/ ./shared/
COPY data-collector/ ./data-collector/
WORKDIR /app/data-collector
RUN go build -o /data-collector ./cmd/main.go

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /data-collector .
CMD ["./data-collector"]
