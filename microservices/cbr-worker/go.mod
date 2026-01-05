module github.com/currency-tracker/go-currency-tracker/microservices/cbr-worker

go 1.21

require (
	github.com/currency-tracker/go-currency-tracker/microservices/shared v0.0.0
	github.com/segmentio/kafka-go v0.4.47
	github.com/sirupsen/logrus v1.9.3
)

require (
	github.com/klauspost/compress v1.17.4 // indirect
	github.com/pierrec/lz4/v4 v4.1.19 // indirect
	golang.org/x/sys v0.13.0 // indirect
)

replace github.com/currency-tracker/go-currency-tracker/microservices/shared => ../shared
