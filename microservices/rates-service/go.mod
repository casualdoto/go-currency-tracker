module github.com/currency-tracker/go-currency-tracker/microservices/rates-service

go 1.21

require (
	github.com/currency-tracker/go-currency-tracker/microservices/shared v0.0.0
	github.com/sirupsen/logrus v1.9.3
)

require golang.org/x/sys v0.13.0 // indirect

replace github.com/currency-tracker/go-currency-tracker/microservices/shared => ../shared
