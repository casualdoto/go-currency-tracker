FROM golang:1.24-alpine AS builder
ENV GOPROXY=https://goproxy.io,direct
ENV CGO_ENABLED=0
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o history-service ./cmd/main.go

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/history-service .
EXPOSE 8084
CMD ["./history-service"]
