FROM golang:1.24-alpine AS builder
ENV GOPROXY=https://proxy.golang.org,direct
ENV CGO_ENABLED=0
WORKDIR /app

COPY shared/go.mod ./shared/
COPY history-service/go.mod history-service/go.sum ./history-service/
WORKDIR /app/history-service
RUN go mod download

WORKDIR /app
COPY shared/ ./shared/
COPY history-service/ ./history-service/
WORKDIR /app/history-service
RUN go build -o /history-service ./cmd/main.go

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /history-service .
EXPOSE 8084
CMD ["./history-service"]
