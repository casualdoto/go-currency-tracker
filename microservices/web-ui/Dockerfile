FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o web-ui ./cmd/main.go

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/web-ui .
COPY --from=builder /app/static ./static
EXPOSE 3000
CMD ["./web-ui"]
