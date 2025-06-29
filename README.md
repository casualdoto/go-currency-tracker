# Go Currency Tracker

Сервис для отслеживания курсов валют ЦБ РФ с REST API.

## Возможности

- Получение курсов всех валют ЦБ РФ
- Получение курса конкретной валюты
- Выбор даты для получения исторических курсов
- OpenAPI документация

## Установка и запуск

### Требования

- Go 1.21+

### Сборка и запуск

```bash
# Клонирование репозитория
git clone https://github.com/casualdoto/go-currency-tracker.git
cd go-currency-tracker

# Сборка проекта
go build -o currency-tracker ./cmd/server

# Запуск сервера
./currency-tracker
```

На Windows:

```cmd
go build -o currency-tracker.exe ./cmd/server
currency-tracker.exe
```

## API

Сервер запускается на порту 8080 по умолчанию.

### Основные эндпоинты

- `GET /ping` - проверка работоспособности API
- `GET /info` - информация о сервисе
- `GET /rates/cbr` - получение всех курсов валют
- `GET /rates/cbr?date=YYYY-MM-DD` - получение всех курсов валют на указанную дату
- `GET /rates/cbr/currency?code=USD` - получение курса доллара США
- `GET /rates/cbr/currency?code=EUR&date=2023-05-15` - получение курса евро на 15 мая 2023 года
- `GET /api/docs` - OpenAPI документация

### Примеры запросов

```bash
# Получение всех курсов валют на текущую дату
curl http://localhost:8080/rates/cbr

# Получение курса доллара США на текущую дату
curl http://localhost:8080/rates/cbr/currency?code=USD

# Получение курса евро на конкретную дату
curl http://localhost:8080/rates/cbr/currency?code=EUR&date=2023-05-15
```

## Документация API

OpenAPI документация доступна по адресу `/api/docs` после запуска сервера.

## Лицензия

MIT
