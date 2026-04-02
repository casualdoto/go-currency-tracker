# Реализация микросервисной архитектуры — план работ

> **Важно:** При реализации микросервисов необходимо ориентироваться на существующую реализацию в папке [`monolith/`](./monolith/). Все бизнес-логика, структуры данных, запросы к внешним API (CBR, Binance), SQL-схемы и паттерны работы с БД берутся из монолита и адаптируются под микросервисную архитектуру. Реализация микросервисов ведётся в папке [`microservices/`](./microservices/).


## Общая архитектура

Монолитное приложение декомпозируется на следующие автономные микросервисы:

| Сервис | Ответственность | БД / Хранилище |
|---|---|---|
| **API Gateway** | Единая точка входа, аутентификация, маршрутизация | — |
| **Auth Service** | Регистрация, вход, токены, сессии | PostgreSQL |
| **Data Collector Service** | Опрос CBR API и Binance API, публикация в Kafka | — |
| **Normalization Service** | Нормализация форматов, конвертация в RUB | — |
| **History Service** | Хранение и выдача исторических данных | PostgreSQL + ClickHouse |
| **Notification Service** | Подписки, триггеры, отправка уведомлений | Redis |
| **Telegram Bot Service** | Взаимодействие с пользователями через Telegram | Redis / in-memory |
| **Web UI Service** | SPA-интерфейс, визуализация курсов | — |

Взаимодействие: **Kafka** (асинхронное) + **REST/gRPC** (синхронное).

---

## TODO List

### 1. Инфраструктура и DevOps

- [x] Настроить Docker-образы для каждого микросервиса
- [x] Написать `docker-compose.yml` для локальной разработки
- [x] Развернуть Kafka (Message Broker) и создать необходимые топики:
  - [x] топик сырых курсов (Data Collector → Normalization)
  - [x] топик нормализованных курсов (Normalization → History, Notification)
- [ ] Настроить CI/CD (GitHub Actions / GitLab CI)
- [ ] (Опционально) Настроить Kubernetes-манифесты для оркестрации

---

### 2. Auth Service (PostgreSQL)

- [x] Создать схему БД:
  - [x] Таблица `users` (id UUID, email, password_hash, created_at)
  - [x] Таблица `user_profiles` (user_id, telegram_id, timezone, language)
  - [x] Таблица `sessions` (token JWT, user_id, expires_at)
- [x] Реализовать эндпоинты:
  - [x] `POST /register` — регистрация
  - [x] `POST /login` — вход, выдача JWT
  - [x] `POST /logout` — инвалидация сессии
  - [x] `GET /validate` — проверка токена (для API Gateway)
- [x] Реализовать хэширование паролей
- [x] Настроить изоляцию БД (только Auth Service имеет доступ)

---

### 3. Data Collector Service

- [x] Реализовать периодический опрос **CBR API** (курсы валют ЦБ РФ)
- [x] Реализовать периодический опрос **Binance API** (OHLCV криптовалют)
- [x] Публиковать сырые данные в Kafka-топик
- [x] Настроить интервалы опроса (минимум — раз в секунду для крипты)
- [x] Обработать ошибки внешних API (retry, circuit breaker)

---

### 4. Normalization Service

- [x] Подписаться на Kafka-топик сырых курсов
- [x] Реализовать нормализацию форматов CBR и Binance к единой схеме
- [x] Реализовать конвертацию криптовалютных курсов в RUB через USDT
- [x] Публиковать нормализованные данные в отдельный Kafka-топик
- [ ] Покрыть логику конвертации юнит-тестами

---

### 5. History Service (PostgreSQL + ClickHouse)

- [x] Создать схему PostgreSQL:
  - [x] Таблица `cbr_rates` (id, date, currency_code, currency_name, nominal, value, previous, created_at)
  - [x] Уникальный индекс по `(date, currency_code)`
  - [x] Индексы по `date` и `currency_code`
- [ ] Создать схему ClickHouse:
  - [ ] Таблица `crypto_rates` (id, timestamp, symbol, open, high, low, close, volume, price_rub, created_at)
  - [ ] Уникальность по `(timestamp, symbol)`
  - [ ] Индексы по `timestamp` и `symbol`
- [x] Подписаться на Kafka-топик нормализованных курсов и сохранять данные
- [x] Реализовать API эндпоинты:
  - [x] Получение курса на конкретную дату
  - [x] Выборка за период
  - [ ] Вычисление агрегатов (min, max, avg)
- [ ] Убедиться в независимом масштабировании сервиса

---

### 6. Notification Service (Redis)

- [x] Проектировать структуру ключей Redis:
  - [x] `user:{id}:subscriptions` — список подписок
  - [ ] `subscription:{id}` — параметры подписки (type, symbol, threshold, frequency)
  - [ ] `notification_log:{user_id}` — журнал уведомлений
  - [ ] TTL и счётчики для ограничения частоты
- [x] Подписаться на Kafka-топик нормализованных курсов
- [x] Реализовать логику проверки триггеров по подпискам пользователей
- [x] При срабатывании — передавать уведомление в **Telegram Bot Service**
- [x] Реализовать API для управления подписками (создание, удаление, список)

---

### 7. Telegram Bot Service

- [x] Реализовать приём команд от пользователей через Telegram API
- [ ] Хранить состояния диалогов:
  - [ ] `dialog_state:{telegram_id}` — текущий шаг взаимодействия
  - [ ] `user_map:{telegram_id}` — маппинг Telegram ID → user_id из Auth Service
- [x] Обращаться к **API Gateway** для получения данных (курсы, история)
- [x] Обращаться к **Notification Service** для управления подписками
- [x] Изолировать сервис от внутренней бизнес-логики — только UI-логика Telegram
- [x] Реализовать обработку основных команд (`/start`, `/rates`, `/subscribe`, `/unsubscribe`, `/history`)

---

### 8. API Gateway

- [x] Реализовать маршрутизацию запросов к микросервисам
- [x] Интегрировать проверку JWT через Auth Service
- [x] Реализовать валидацию входящих запросов
- [x] Настроить маршруты:
  - [x] `/history/*` → History Service
  - [x] `/notifications/*` → Notification Service
  - [x] `/auth/*` → Auth Service
- [ ] Настроить rate limiting

---

### 9. Web UI Service (SPA)

- [x] Реализовать визуализацию текущих курсов валют и криптовалют
- [x] Реализовать исторические графики
- [x] Реализовать управление уведомлениями (подписки)
- [x] Реализовать экспорт данных
- [x] Все запросы — только через API Gateway



---

### 10. Тестирование и интеграция

- [x] Написать юнит-тесты для каждого сервиса
- [ ] Написать интеграционные тесты для межсервисного взаимодействия
- [ ] Проверить сценарий end-to-end: CBR/Binance → Kafka → History → API → Web UI
- [ ] Проверить сценарий уведомлений: курс срабатывает → Notification → Telegram Bot
- [ ] Нагрузочное тестирование ключевых сервисов

### 11. Перенос таблицы из Postgres в Clickhouse

- [x] Создать схему ClickHouse (и удалить из Postgres):
  - [x] Таблица `crypto_rates` (timestamp, symbol, open, high, low, close, volume, price_rub, created_at)
  - [x] Уникальность по `(timestamp, symbol)` через ReplacingMergeTree ORDER BY
  - [x] Индексы по `timestamp` и `symbol` через ORDER BY ключ


---

## Стек технологий

| Категория | Технологии |
|---|---|
| Язык | Go (goroutines, channels) |
| API | REST, gRPC, WebSocket |
| Очереди | Kafka, RabbitMQ |
| БД | PostgreSQL, ClickHouse, Redis |
| Контейнеризация | Docker |
| Оркестрация | Kubernetes |
| CI/CD | GitHub Actions / GitLab CI / Jenkins |
| Фреймворки | Gin, Echo, gRPC, Kafka-client, Redis-client |
