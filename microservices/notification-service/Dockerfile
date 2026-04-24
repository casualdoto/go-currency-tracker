FROM golang:1.23-alpine AS builder
ENV GOPROXY=https://proxy.golang.org,direct
ENV CGO_ENABLED=0
WORKDIR /app

COPY shared/go.mod ./shared/
COPY notification-service/go.mod notification-service/go.sum ./notification-service/
WORKDIR /app/notification-service
RUN go mod download

WORKDIR /app
COPY shared/ ./shared/
COPY notification-service/ ./notification-service/
WORKDIR /app/notification-service
RUN go build -o /notification-service ./cmd/main.go

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /notification-service .
EXPOSE 8085
CMD ["./notification-service"]
