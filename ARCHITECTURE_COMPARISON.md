# Монолит vs Микросервисы: Сравнение архитектур

Детальный анализ двух параллельных реализаций проекта go-currency-tracker.

---

## Содержание

1. [Общий обзор](#1-общий-обзор)
2. [Поток данных: Монолит](#2-поток-данных-монолит)
3. [Поток данных: Микросервисы](#3-поток-данных-микросервисы)
4. [Разбор каждого сервиса](#4-разбор-каждого-сервиса)
5. [Сбор курсов ЦБ РФ](#5-сбор-курсов-цб-рф)
6. [Сбор курсов криптовалют](#6-сбор-курсов-криптовалют)
7. [Нормализация курсов](#7-нормализация-курсов)
8. [Хранение данных](#8-хранение-данных)
9. [API-слой](#9-api-слой)
10. [Telegram-бот](#10-telegram-бот)
11. [Аутентификация и авторизация](#11-аутентификация-и-авторизация)
12. [Планировщик и фоновые задачи](#12-планировщик-и-фоновые-задачи)
13. [Развёртывание и инфраструктура](#13-развёртывание-и-инфраструктура)
14. [Примеры прохождения запросов](#14-примеры-прохождения-запросов)
15. [Обработка ошибок и отказоустойчивость](#15-обработка-ошибок-и-отказоустойчивость)
16. [Тестирование](#16-тестирование)
17. [Итоговая сравнительная таблица](#17-итоговая-сравнительная-таблица)

---

## 1. Общий обзор

### Монолит (`monolith/`)

Однопроцессное Go-приложение. Вся бизнес-логика (HTTP API, Telegram-бот, планировщик, доступ к базе данных) находится в одной кодовой базе и разворачивается как один или два бинарных файла (`cmd/server` и `cmd/bot`), использующих общую базу данных PostgreSQL.

```
monolith/
├── cmd/
│   ├── server/      → HTTP API сервер  (порт 8081)
│   └── bot/         → Telegram-бот (отдельный бинарник)
└── internal/
    ├── api/         → HTTP-обработчики и маршрутизация
    ├── currency/
    │   ├── cbr/     → Клиент ЦБ РФ (XML/JSON)
    │   └── binance/ → Клиент Binance API + конвертация в рубли
    ├── storage/     → Слой PostgreSQL (все таблицы)
    ├── scheduler/   → Ежедневные и 15-минутные задачи (внутри процесса)
    ├── alert/       → Логика Telegram-бота + подписки
    └── config/      → Конфигурация на основе godotenv
```

**Инфраструктура:** 1× PostgreSQL для всего.

---

### Микросервисы (`microservices/`)

Восемь независимых Go-сервисов, взаимодействующих через **Kafka** (асинхронно) и **HTTP** (синхронно). Каждый сервис владеет собственным хранилищем данных и разворачивается как отдельный Docker-контейнер.

```
microservices/
├── data-collector        → Опрашивает ЦБ РФ + Binance, публикует в Kafka
├── normalization-service → Потребляет сырые события, нормализует, переиздаёт
├── history-service       → Сохраняет курсы; предоставляет HTTP API истории
├── auth-service          → Регистрация пользователей, выдача/проверка JWT
├── notification-service  → Подписки в Redis + Kafka-driven отправка в Telegram
├── api-gateway           → Обратный прокси + JWT middleware
├── telegram-bot          → Telegram-бот (вызывает HTTP-сервисы)
├── web-ui                → Сервер статических файлов (порт 3000)
└── shared/events/        → Общие типы Kafka-событий (Go workspace)
```

**Инфраструктура:** 2× PostgreSQL (auth, history), 1× ClickHouse, 1× Redis, 1× Kafka + Zookeeper.

---

## 2. Поток данных: Монолит

### Курсы ЦБ РФ (фиатные)

```
                                ┌─────────────────┐
                                │  Планировщик    │  запускается ежедневно в 23:59 UTC
                                │  (внутри процесса)│
                                └────────┬────────┘
                                         │ вызывает
                                         ▼
                              currency.GetCBRRates()
                                         │ HTTP GET (таймаут 10с)
                                         ▼
                            https://www.cbr-xml-daily.ru
                            /daily_json.js  (JSON)
                                         │
                                         ▼
                            Parse → []CurrencyRate
                                         │
                                         ▼
                            db.SaveCurrencyRates()  ──► PostgreSQL
                                                        currency_rates
                                                        UNIQUE(date, code)
```

**При API-запросе (`GET /rates/cbr?date=2024-01-15`):**

```
HTTP-клиент
    │ GET /rates/cbr?date=2024-01-15
    ▼
chi Router → CBRRatesHandler
    │
    ├─► db.GetCurrencyRatesByDate(date)  ──► PostgreSQL (есть ли в кэше?)
    │       │
    │       ├── ЕСТЬ  → немедленно вернуть JSON
    │       │
    │       └── НЕТ → currency.GetCBRRatesByDate(date)
    │                       │ HTTP GET /archive/2024/01/15/daily_json.js
    │                       ▼
    │                   Parse JSON → []CurrencyRate
    │                       │
    │                       └── go db.SaveCurrencyRates() (асинхронно, без блокировки)
    │
    └─► вернуть JSON  {"success": true, "data": {map of Valute}}
```

### Курсы криптовалют

```
HTTP-клиент
    │ GET /rates/crypto/history?symbol=BTC&days=7
    ▼
chi Router → GetCryptoHistoryHandler
    │
    ├── для каждого из 7 дней:
    │       │
    │       ├─► db.GetCryptoRatesByDateRange()  ──► PostgreSQL
    │       │       │
    │       │       ├── ЕСТЬ  → использовать закэшированный OHLCV
    │       │       │
    │       │       └── НЕТ → binance.GetHistoricalCryptoToRubRates()
    │       │                       │ запрос klines symbolUSDT
    │       │                       │ запрос klines USDTRUB  (или fallback ЦБ РФ USD)
    │       │                       │ вычислить crypto/RUB для каждой свечи
    │       │                       │ повтор до 3× (экспон. backoff: 1с/2с/4с)
    │       │                       ▼
    │       │                   []CryptoRate (OHLCV + RUB)
    │       │                       │
    │       │                       └── go db.SaveCryptoRates() (асинхронно)
    │       │
    │       └── накопить результат дня
    │
    └─► вернуть JSON [{timestamp, open, high, low, close, volume}]
```

### Telegram-уведомления

```
TelegramScheduler (горутина внутри процесса)
    │
    ├── каждые 15 минут:
    │       │
    │       ├── для каждого пользователя в cryptoSubs (in-memory map):
    │       │       │
    │       │       └── binance.GetCurrentCryptoToRubRate(symbol)
    │       │               │ HTTP-запрос к Binance
    │       │               ▼
    │       │           price float64
    │       │               │
    │       │               ├── сравнить с lastCryptoPrices[symbol]
    │       │               │
    │       │               └── если изменилась → bot.Send(user, "📈 BTC: 4500000 RUB")
    │       │
    │       └── для каждого пользователя в subscriptions (фиат ЦБ РФ):
    │               └── отправить ежедневное обновление ЦБ РФ в 2:00 UTC
    │
    └── подписки загружаются из PostgreSQL при старте
        и хранятся в памяти map[int][]string
```

---

## 3. Поток данных: Микросервисы

### Полный пайплайн (ЦБ РФ + Крипто → Хранение → Уведомления)

```
┌─────────────────────────────────────────────────────────────────────┐
│  СЛОЙ СБОРА ДАННЫХ                                                  │
│                                                                     │
│  data-collector                                                     │
│  ├── CBRCollector.Collect()      каждые 86400с (1 день)             │
│  │       │ HTTP GET /daily_json.js (таймаут 15с)                    │
│  │       ▼                                                          │
│  │   RawCBRRatesEvent{source:"cbr", rates:[]}                       │
│  │       │ JSON encode                                              │
│  │       ▼                                                          │
│  │   Kafka топик: raw-rates  (3 партиции)                           │
│  │                                                                  │
│  └── CryptoCollector.Collect()   каждые 60с                         │
│          │ Binance ListPriceChangeStats (10 USDT-пар)               │
│          ▼                                                          │
│      RawCryptoRatesEvent{source:"binance", rates:[]}                │
│          │ JSON encode                                              │
│          ▼                                                          │
│      Kafka топик: raw-rates  (3 партиции)                           │
└─────────────────────────────────────────────────────────────────────┘
                          │
                          │  Kafka consumer (group: normalization-service)
                          ▼
┌─────────────────────────────────────────────────────────────────────┐
│  СЛОЙ НОРМАЛИЗАЦИИ                                                  │
│                                                                     │
│  normalization-service                                              │
│  │                                                                  │
│  ├── source == "cbr":                                               │
│  │       парсинг даты (учёт особенностей ЦБ РФ API)                 │
│  │       формирование NormalizedCBRRatesEvent{rates:[               │
│  │           {Date, CurrencyCode, CurrencyName, Nominal,            │
│  │            Value, Previous}                                      │
│  │       ]}                                                         │
│  │       → Kafka топик: normalized-rates                            │
│  │                                                                  │
│  └── source == "binance":                                           │
│          HTTP GET cbr-xml-daily.ru  (получить текущий USD/RUB)      │
│          PriceRUB = Close(USDT) × USD_RUB_rate                     │
│          формирование NormalizedCryptoRatesEvent{rates:[            │
│              {Symbol, OpenRUB, HighRUB, LowRUB, CloseRUB,          │
│               VolumeUSDT, PriceRUB, Timestamp}                     │
│          ]}                                                         │
│          → Kafka топик: normalized-rates                            │
└─────────────────────────────────────────────────────────────────────┘
                          │
                          │  Kafka fan-out: 2 независимые consumer group
                          ▼
        ┌─────────────────────────────────────────┐
        │                                         │
        ▼                                         ▼
┌───────────────────┐                  ┌──────────────────────────┐
│  history-service  │                  │  notification-service    │
│                   │                  │                          │
│  CBR событие:     │                  │  Крипто событие:         │
│  INSERT cbr_rates │                  │  убрать суффикс USDT     │
│  → PostgreSQL     │                  │  найти в Redis:          │
│  (ON CONFLICT DO  │                  │  crypto:{telegram_id}    │
│   UPDATE)         │                  │  → список символов       │
│                   │                  │                          │
│  Крипто событие:  │                  │  для каждого подписчика: │
│  INSERT INTO      │                  │  POST Telegram API       │
│  crypto_rates     │                  │  "💰 BTC: 4500000 RUB"  │
│  → ClickHouse     │                  │                          │
│  (ReplacingMerge  │                  └──────────────────────────┘
│   Tree)           │
└───────────────────┘
```

### Синхронный HTTP-путь (клиентский запрос)

```
Браузер / Telegram-бот
        │
        │ HTTP запрос
        ▼
┌───────────────────────────────────────────────────────┐
│  api-gateway  :8080                                   │
│                                                       │
│  Маршрутизация:                                       │
│  /auth/*           → proxy → auth-service:8082        │
│  /rates/cbr        → proxy → history-service:8084     │
│  /rates/crypto/*   → proxy → history-service:8084     │
│  /history/*        → proxy → history-service:8084     │
│  /notifications/*  → [JWT проверка] → proxy →         │
│                      notification-service:8085        │
│                                                       │
│  JWT Middleware (для защищённых маршрутов):           │
│  1. Извлечь Bearer токен из заголовка Authorization   │
│  2. GET auth-service:8082/validate?token=...          │
│  3. Если 401 → немедленно отклонить                   │
│  4. Если 200 → передать оригинальный запрос           │
└───────────────────────────────────────────────────────┘
        │                              │
        ▼ (публичный)                  ▼ (защищённый)
┌─────────────────┐          ┌──────────────────────┐
│ history-service │          │ notification-service  │
│                 │          │                       │
│ GET /history/   │          │ POST/DELETE           │
│  cbr?date=...   │          │  /subscriptions/cbr   │
│                 │          │  /subscriptions/crypto│
│ 1. Запрос в PG  │          │                       │
│ 2. Если пусто:  │          │ Чтение/запись Redis   │
│    cbrbackfill  │          │ Вернуть 200 OK        │
│    → архив ЦБ   │          └──────────────────────┘
│    → сохранить  │
│ 3. Вернуть JSON │
│                 │
│ GET /history/   │
│  crypto/range   │
│                 │
│ если диапазон   │
│  ≤ 7 дней:      │
│   cryptobackfill│
│   → Binance API │
│   → сохр. CH    │
│ иначе:          │
│   ClickHouse    │
│   range query   │
│ 3. Вернуть JSON │
└─────────────────┘
```

### Поток аутентификации

```
Клиент
    │ POST /auth/register  {"email":"a@b.com","password":"secret"}
    ▼
api-gateway → auth-service
    │
    ├── bcrypt.Hash(password, cost=10)
    ├── INSERT INTO users (email, password_hash)
    ├── INSERT INTO sessions (token, user_id, expires_at=now+24h)
    └── вернуть {"token": "eyJhbGci..."}

Клиент
    │ POST /auth/login
    ▼
api-gateway → auth-service
    │
    ├── SELECT password_hash FROM users WHERE email = ?
    ├── bcrypt.CompareHashAndPassword(hash, password)
    ├── jwt.Sign({sub, email, exp=now+24h}, HS256, JWT_SECRET)
    ├── INSERT INTO sessions (token, user_id, expires_at)
    └── вернуть {"token": "eyJhbGci..."}

Клиент
    │ GET /notifications/subscriptions  (Authorization: Bearer eyJ...)
    ▼
api-gateway JWT middleware
    │
    ├── GET auth-service/validate?token=eyJ...
    │       │
    │       ├── SELECT user_id, expires_at FROM sessions WHERE token = ?
    │       ├── если не найден ИЛИ истёк → 401
    │       └── 200 OK {"user_id": "uuid"}
    │
    └── передать запрос в notification-service
```

---

## 4. Разбор каждого сервиса

### Карта архитектуры микросервисов

```
                           ┌─────────────┐
                    ┌─────►│ auth-service│◄──── postgres-auth:5432
                    │      │   :8082     │      (users, sessions)
                    │      └─────────────┘
                    │
┌──────────┐  HTTP  │      ┌─────────────┐
│  Клиент  │───────►│      │ api-gateway │
│ Браузер/ │        │      │   :8080     │
│ Бот и др.│        └─────►│             │
└──────────┘               │  JWT proxy  │
                    ┌─────►│             │
                    │      └──────┬──────┘
                    │             │ HTTP proxy
                    │      ┌──────▼──────┐
                    │      │history-svc  │◄──── postgres-history:5433
                    │      │   :8084     │      (cbr_rates)
                    │      │             │◄──── clickhouse:9000
                    │      └─────────────┘      (crypto_rates)
                    │
                    │      ┌─────────────┐
                    └─────►│notif-service│◄──── redis:6379
                           │   :8085     │      (subscriptions)
                           └──────▲──────┘
                                  │ Kafka consumer
                                  │
                    ┌─────────────┴────────────────────────┐
                    │         Kafka: normalized-rates       │
                    └──────────────────┬───────────────────┘
                                       │ consumer
                              ┌────────▼────────┐
                              │  normalization  │
                              │    -service     │
                              └────────▲────────┘
                                       │ Kafka consumer
                    ┌──────────────────┴───────────────────┐
                    │           Kafka: raw-rates            │
                    └──────────────────▲───────────────────┘
                                       │ producer
                              ┌────────┴────────┐
                              │ data-collector  │◄── ЦБ РФ API
                              │                 │◄── Binance API
                              └─────────────────┘

                              ┌─────────────────┐
                              │  telegram-bot   │──► Telegram API
                              │                 │
                              │  вызывает:      │
                              │  api-gateway    │
                              │  notif-service  │
                              └─────────────────┘

                              ┌─────────────────┐
                              │    web-ui       │
                              │    :3000        │──► api-gateway
                              └─────────────────┘
```

---

## 5. Сбор курсов ЦБ РФ

### Монолит (`internal/currency/cbr/cbr.go`)

- **`GetCBRRates()`** — одиночный HTTP GET на `/daily_json.js`, таймаут 10с, возвращает `DailyRates{Valute map[string]Valute}`
- **`GetCBRRatesByDate(date)`** — формирует URL `/archive/YYYY/MM/DD/daily_json.js`, парсинг идентичный
- Вызывается **по требованию** HTTP-обработчиками (паттерн cache-aside):
  1. Сначала проверить PostgreSQL
  2. На промахе кэша → вызов ЦБ РФ API → асинхронное сохранение → ответ клиенту
- Также вызывается **планировщиком** раз в день в 23:59 UTC для проактивного прогрева кэша

### Микросервисы (`data-collector/internal/collector/cbr.go`)

- **`CBRCollector.Collect()`** — HTTP GET `/daily_json.js`, таймаут 15с
- Запускается **по фиксированному расписанию** (по умолчанию 86400с), полностью отвязан от HTTP-запросов
- **Не пишет** ни в какую базу данных — вместо этого публикует в Kafka:
  ```json
  {
    "source": "cbr",
    "collected_at": "2024-01-15T23:59:00Z",
    "rates": [
      {"CharCode": "USD", "Name": "US Dollar", "Nominal": 1, "Value": 89.50, "Previous": 89.12}
    ]
  }
  ```
- Сохранением занимаются `normalization-service` и далее `history-service`

**Ключевое отличие:** монолит забирает данные по требованию + cache-aside; микросервисы публикуют события по расписанию вне зависимости от клиентских запросов.

---

## 6. Сбор курсов криптовалют

### Монолит (`internal/currency/binance/binance.go`)

**Текущий курс:**
```
binance.GetCurrentCryptoToRubRate("BTC")
    │
    ├── получить klines BTCUSDT (интервал 1ч, текущая свеча)
    ├── получить klines USDTRUB (1ч)
    │       └── fallback: ЦБ РФ API для USD/RUB, если Binance недоступен
    └── вернуть Close(BTCUSDT) × Close(USDTRUB)
```

**История (диапазон 7 дней, свечи 15м):**
```
binance.GetHistoricalCryptoToRubRates("BTC", "15m", start, end)
    │
    ├── получить все klines BTCUSDT за диапазон
    ├── получить все klines USDTRUB за диапазон
    │       └── fallback: ежедневные курсы USD ЦБ РФ (поиск по дням)
    ├── соединить по timestamp (пропустить, если разрыв > 1 часа)
    ├── вычислить: close_usdt × usdt_rub = close_rub  (для каждой свечи)
    └── вернуть []CryptoRate{open_rub, high_rub, low_rub, close_rub, volume}
```

Логика повторных попыток: до 3 раз с экспоненциальным backoff (1с → 2с → 4с).

### Микросервисы

**Сбор** (`data-collector/internal/collector/crypto.go`):
- Использует Binance `ListPriceChangeStats` (тикер за 24ч) для 10 жёстко заданных символов
- Запускается каждые 60 секунд
- Публикует сырые цены **только в USDT** — конвертации в рубли на этом этапе нет:
  ```json
  {"source":"binance","rates":[{"Symbol":"BTCUSDT","Close":42000.5,"Volume":1234.5,...}]}
  ```

**Конвертация в рубли** (`normalization-service/internal/normalizer/normalizer.go`):
- Обращается к ЦБ РФ API **на каждое крипто-сообщение** для получения актуального USD/RUB
- `PriceRUB = Close(USDT) × USD_RUB`
- Курс конвертации берётся в реальном времени (без кэша), что обеспечивает актуальность расчёта

**Бэкфил** (`history-service/internal/cryptobackfill/`):
- Для коротких диапазонов (≤7 дней): `FetchIntervalRUBRates()` напрямую вызывает Binance klines + ЦБ РФ
- `IntervalForCalendarSpan(days)` сопоставляет диапазон с разрешением (например, 7д → 15м)
- После загрузки сохраняет в ClickHouse

---

## 7. Нормализация курсов

### Монолит

Отдельного слоя нормализации нет. Конвертация происходит прямо в HTTP-обработчиках:
- `cbr_handlers.go`: JSON ЦБ РФ → `storage.CurrencyRate` перед записью в БД
- `crypto_handlers.go`: структура Binance → `storage.CryptoRate` перед записью в БД
- Парсинг дат и валидация смешаны с логикой обработчика

### Микросервисы (`normalization-service/internal/normalizer/normalizer.go`)

Выделенный сервис-потребитель Kafka, который одновременно является продюсером:

```
Читать сообщение из raw-rates
    │
    ├── source == "cbr" → normalizeCBR()
    │       │ парсинг даты: пробует несколько форматов
    │       │   "2006-01-02T15:04:05Z07:00"
    │       │   "2006-01-02"
    │       │   убирает trailing 'Z' и т.д.
    │       ▼
    │   NormalizedCBRRatesEvent{
    │       Date: time.Time
    │       Rates: [{Code, Name, Nominal, Value, Previous}]
    │   }
    │   → опубликовать в normalized-rates
    │
    └── source == "binance" → normalizeCrypto()
            │ HTTP GET cbr-xml-daily.ru → курс USD/RUB
            │ для каждого сырого курса:
            │   priceRUB = close * usdRub
            │   openRUB  = open  * usdRub
            │   highRUB  = high  * usdRub
            │   lowRUB   = low   * usdRub
            ▼
        NormalizedCryptoRatesEvent{
            Rates: [{Symbol, OpenRUB, HighRUB, LowRUB, CloseRUB,
                     VolumeUSDT, PriceRUB, Timestamp}]
        }
        → опубликовать в normalized-rates
```

**Типы Kafka-событий** (`shared/events/events.go`):
```go
// топик raw-rates
RawCBRRatesEvent    { Source, CollectedAt, Rates []RawCBRRate    }
RawCryptoRatesEvent { Source, CollectedAt, Rates []RawCryptoRate }

// топик normalized-rates
NormalizedCBRRatesEvent    { Date, Rates []NormalizedCBRRate    }
NormalizedCryptoRatesEvent { Rates []NormalizedCryptoRate       }
```

---

## 8. Хранение данных

### Монолит — PostgreSQL (один экземпляр)

```sql
-- Курсы фиатных валют ЦБ РФ
CREATE TABLE currency_rates (
    id            SERIAL PRIMARY KEY,
    date          DATE          NOT NULL,
    currency_code VARCHAR(3)    NOT NULL,
    currency_name VARCHAR(100)  NOT NULL,
    nominal       INTEGER       NOT NULL,
    value         DECIMAL(12,4) NOT NULL,
    previous      DECIMAL(12,4),
    created_at    TIMESTAMPTZ   DEFAULT NOW(),
    UNIQUE (date, currency_code)
);
CREATE INDEX idx_currency_rates_date ON currency_rates(date);
CREATE INDEX idx_currency_rates_code ON currency_rates(currency_code);

-- OHLCV криптовалют с Binance (хранятся в рублях)
CREATE TABLE crypto_rates (
    id         SERIAL PRIMARY KEY,
    timestamp  BIGINT        NOT NULL,   -- Unix мс
    symbol     VARCHAR(20)   NOT NULL,
    open       DECIMAL(24,8) NOT NULL,   -- значения в рублях
    high       DECIMAL(24,8) NOT NULL,
    low        DECIMAL(24,8) NOT NULL,
    close      DECIMAL(24,8) NOT NULL,
    volume     DECIMAL(24,8) NOT NULL,
    created_at TIMESTAMPTZ   DEFAULT NOW(),
    UNIQUE (timestamp, symbol)
);
CREATE INDEX idx_crypto_rates_timestamp ON crypto_rates(timestamp);
CREATE INDEX idx_crypto_rates_symbol    ON crypto_rates(symbol);

-- Telegram-подписки (фиат)
CREATE TABLE telegram_subscriptions (
    telegram_id   BIGINT      NOT NULL,
    currency_code VARCHAR(10) NOT NULL,
    PRIMARY KEY (telegram_id, currency_code)
);

-- Telegram-подписки (крипто)
CREATE TABLE telegram_crypto_subscriptions (
    telegram_id BIGINT      NOT NULL,
    symbol      VARCHAR(20) NOT NULL,
    PRIMARY KEY (telegram_id, symbol)
);
```

**Паттерны доступа:** upsert при записи (`ON CONFLICT DO UPDATE`), range scan при чтении.

---

### Микросервисы — три хранилища данных

#### PostgreSQL `auth_db` (auth-service, порт 5432)

```sql
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255)        NOT NULL,
    created_at    TIMESTAMPTZ         DEFAULT NOW()
);

CREATE TABLE user_profiles (
    user_id     UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    telegram_id BIGINT,
    timezone    VARCHAR(64) DEFAULT 'UTC',
    language    VARCHAR(10) DEFAULT 'en'
);

CREATE TABLE sessions (
    token      TEXT PRIMARY KEY,
    user_id    UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ  NOT NULL,
    created_at TIMESTAMPTZ  DEFAULT NOW()
);
CREATE INDEX idx_sessions_user_id    ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
```

#### PostgreSQL `history_db` (history-service, порт 5433)

```sql
CREATE TABLE cbr_rates (
    id            SERIAL PRIMARY KEY,
    date          DATE          NOT NULL,
    currency_code VARCHAR(3)    NOT NULL,
    currency_name VARCHAR(100)  NOT NULL,
    nominal       INTEGER       NOT NULL,
    value         DECIMAL(12,4) NOT NULL,
    previous      DECIMAL(12,4),
    created_at    TIMESTAMPTZ   DEFAULT NOW(),
    UNIQUE (date, currency_code)
);
CREATE INDEX idx_cbr_rates_date ON cbr_rates(date);
CREATE INDEX idx_cbr_rates_code ON cbr_rates(currency_code);
```

#### ClickHouse (history-service, порт 9000)

```sql
CREATE TABLE IF NOT EXISTS crypto_rates (
    timestamp DateTime,
    symbol    String,
    open      Float64,
    high      Float64,
    low       Float64,
    close     Float64,
    volume    Float64,
    price_rub Float64,
    created_at DateTime DEFAULT now()
)
ENGINE = ReplacingMergeTree(created_at)
ORDER BY (symbol, timestamp);
```

`ReplacingMergeTree` дедуплицирует по `(symbol, timestamp)` в фоновых мержах. Оптимизирован для высокой частоты вставок временных рядов — значительно эффективнее PostgreSQL для миллионов 15-минутных крипто-свечей.

#### Redis (notification-service)

```
Схема ключей:
  cbr:<telegram_id>    → SET кодов валют       e.g. {"USD", "EUR", "CNY"}
  crypto:<telegram_id> → SET символов          e.g. {"BTCUSDT", "ETHUSDT"}
```

Redis обеспечивает проверку членства в множестве за O(1) и быстрый перебор всех подписчиков при поступлении новой цены криптовалюты.

---

## 9. API-слой

### Монолит (`internal/api/`)

Все эндпоинты **публичны** (без аутентификации).

| Метод | Путь | Обработчик | Описание |
|--------|------|---------|-------------|
| GET | `/ping` | inline | Проверка работоспособности |
| GET | `/info` | inline | Информация о версии сервиса |
| GET | `/rates/cbr` | `CBRRatesHandler` | Текущие или исторические курсы ЦБ РФ |
| GET | `/rates/cbr/currency` | `CBRCurrencyHandler` | Курс одной валюты |
| GET | `/rates/crypto/symbols` | `GetAvailableCryptoSymbolsHandler` | Уникальные символы в БД |
| GET | `/rates/crypto/history` | `GetCryptoHistoryHandler` | Крипто за последние N дней |
| GET | `/rates/crypto/history/range` | `GetCryptoHistoryByDateRangeHandler` | За диапазон дат |
| GET | `/rates/crypto/history/range/excel` | `ExportCryptoHistoryToExcelHandler` | Скачать .xlsx |
| GET | `/api/docs` | Swagger UI | API-документация |
| GET | `/` | Static | Web UI |

Формат ответа: обёртка `{"success": true, "data": {...}}`.

### Микросервисы — api-gateway (`internal/gateway/gateway.go`)

Единая точка входа для всех клиентов. Маршрутизирует по префиксу пути, проверяет JWT для защищённых маршрутов.

| Префикс | Целевой сервис | Требуется JWT |
|--------|---------------|---------------|
| `/auth/*` | auth-service:8082 | Нет |
| `/rates/*` | history-service:8084 | Нет |
| `/history/*` | history-service:8084 | Нет |
| `/notifications/*` | notification-service:8085 | Да |
| `/ping` | сам gateway | Нет |

**Таймаут прокси gateway:** 120 секунд (позволяет выполнять медленные запросы к ClickHouse и бэкфил).

**Эндпоинты history-service** (вызываются через gateway):

| Метод | Путь | Описание |
|--------|------|-------------|
| GET | `/history/cbr` | Курсы ЦБ РФ за дату (бэкфил при отсутствии) |
| GET | `/history/cbr/range` | Диапазон ЦБ РФ: `?code=USD&from=...&to=...` |
| GET | `/history/crypto` | Последние N записей крипто из ClickHouse |
| GET | `/history/crypto/range` | Диапазон крипто; бэкфил для ≤7 дней |

**Эндпоинты auth-service:**

| Метод | Путь | Описание |
|--------|------|-------------|
| POST | `/register` | Создать аккаунт, вернуть JWT |
| POST | `/login` | Аутентификация, вернуть JWT |
| POST | `/logout` | Инвалидировать сессию |
| GET | `/validate` | Проверить токен (вызывается gateway) |

**Эндпоинты notification-service:**

| Метод | Путь | Описание |
|--------|------|-------------|
| POST | `/subscriptions/cbr` | Подписаться на валюту ЦБ РФ |
| DELETE | `/subscriptions/cbr` | Отписаться |
| POST | `/subscriptions/crypto` | Подписаться на крипто-символ |
| DELETE | `/subscriptions/crypto` | Отписаться |
| GET | `/subscriptions` | Список подписок пользователя |

---

## 10. Telegram-бот

### Монолith (`cmd/bot/main.go` + `internal/alert/telegram.go`)

Отдельный бинарник; **напрямую вызывает внутренние пакеты** — никакого HTTP.

```
TelegramBot struct:
├── db              *storage.PostgresDB      (общая с сервером)
├── cbr             *cbr.CBRClient
├── binance         *binance.Client
├── subscriptions   map[int][]string         (в памяти, загружается из БД)
├── cryptoSubs      map[int][]string         (в памяти, загружается из БД)
└── lastCryptoPrices map[string]float64      (детектирование изменения цены)
```

**Обработка команд:**

| Команда | Что происходит |
|---------|-------------|
| `/start` | Отправить справочное сообщение |
| `/currencies` | `cbr.GetCBRRates()` → форматировать список |
| `/subscribe USD` | Добавить в in-memory map + `db.SaveSubscription()` |
| `/rate USD` | `cbr.GetCBRRates()` → найти USD → отправить цену |
| `/cryptocurrencies` | Отправить жёстко заданный список из 20 символов |
| `/crypto_subscribe BTC` | Добавить в in-memory map + `db.SaveCryptoSubscription()` |
| `/crypto_rate BTC` | `binance.GetCurrentCryptoToRubRate("BTC")` → отправить цену |

Подписки переживают рестарт через PostgreSQL, но хранятся в памяти для быстрого доступа.

**Уведомления на основе планировщика (в том же процессе):**
- 2:00 UTC: ежедневная отправка курсов ЦБ РФ всем фиатным подписчикам
- Каждые 15 мин: отправка крипто-цены при изменении по сравнению с предыдущим опросом

### Микросервисы (`telegram-bot/internal/bot/bot.go`)

Отдельный сервис; **вызывает только HTTP API** — без прямого доступа к БД или API-клиентам.

```
Bot struct:
├── apiGatewayURL     string   (http://api-gateway:8080)
└── notificationURL   string   (http://notification-service:8085)
```

**Обработка команд:**

| Команда | HTTP-вызов |
|---------|---------------|
| `/start` | — (локальный ответ) |
| `/rates` | GET `{gateway}/rates/cbr` |
| `/subscribe USD` | POST `{notification}/subscriptions/cbr` `{telegram_id, value:"USD"}` |
| `/history USD` | GET `{gateway}/history/cbr/range?code=USD&from=...&to=...` |
| `/crypto_subscribe BTC` | POST `{notification}/subscriptions/crypto` `{telegram_id, value:"BTC"}` |
| `/crypto_unsubscribe BTC` | DELETE `{notification}/subscriptions/crypto` |

**Бот не отправляет уведомления.** Этим занимается `notification-service` при потреблении Kafka-событий:
```
Kafka: normalized-rates (крипто-событие)
    → подписчик notification-service
    → Redis: какие telegram_id подписаны на этот символ?
    → POST Telegram Bot API для каждого подписчика
```

**Сравнение:**

| Аспект | Монолит (бот) | Микросервисы (бот) |
|--------|-------------|------------------|
| Хранение подписок | PostgreSQL + in-memory map | Только Redis |
| Триггер уведомлений | Горутина-планировщик (опрос каждые 15 мин) | Kafka event push (обновление ~60с) |
| Получение цены крипто | Прямой вызов Binance API | Предвычислено normalization-service |
| Связанность | Тесная — с БД и API-клиентами | Слабая — через HTTP |

---

## 11. Аутентификация и авторизация

### Монолит

**Отсутствует.** Все HTTP-эндпоинты публичны. Telegram-бот использует `m.Sender.ID` напрямую — без верификации.

### Микросервисы (auth-service)

Полноценная JWT-аутентификация:

```
Регистрация:
  bcrypt.Hash(password, cost=10) → password_hash
  INSERT INTO users
  INSERT INTO sessions (expires 24h)
  jwt.Sign({sub:uuid, email, exp:unix}) HS256
  → вернуть token

Вход:
  SELECT password_hash FROM users
  bcrypt.CompareHashAndPassword()
  jwt.Sign(claims)
  INSERT INTO sessions
  → вернуть token

Валидация (вызывается API Gateway):
  SELECT expires_at FROM sessions WHERE token = ?
  если не найден → 401
  если time.Now().After(expires_at) → 401
  → 200 OK

Выход:
  DELETE FROM sessions WHERE token = ?
```

**Срок жизни токена:** 24 часа.  
**Алгоритм:** HS256 с переменной окружения `JWT_SECRET`.  
**Refresh-токенов нет** — после истечения срока пользователь должен войти заново.

---

## 12. Планировщик и фоновые задачи

### Монолит (`internal/scheduler/`)

Два планировщика работают как **горутины внутри серверного процесса**:

**CurrencyRateScheduler:**
```go
// Запускается ежедневно в указанное UTC время (час:минута)
func (s *CurrencyRateScheduler) Start() {
    now := time.Now().UTC()
    next := вычисление времени следующего запуска
    time.AfterFunc(next, func() {
        s.updateCurrencyRates()   // получить курсы ЦБ РФ + сохранить в БД
        ticker := time.NewTicker(24 * time.Hour)
        for range ticker.C {
            s.updateCurrencyRates()
        }
    })
}
```

**TelegramScheduler:**
```go
// Ежедневные обновления ЦБ РФ в 2:00 UTC всем фиатным подписчикам
// Крипто-обновления каждые 15 минут с детектированием изменения
go func() {
    ticker15m := time.NewTicker(15 * time.Minute)
    ticker24h  := time.NewTicker(24 * time.Hour)
    for {
        select {
        case <-ticker15m.C: sendCryptoUpdates()
        case <-ticker24h.C: sendCurrencyUpdates()
        }
    }
}()
```

**Плюсы:** просто, нулевая инфраструктура.  
**Минусы:** привязан к аптайму сервера; при рестарте планировщик сбрасывается; нельзя масштабировать независимо.

### Микросервисы

Планировщиков нет. `data-collector` выполняет **непрерывные циклы**:

```go
func main() {
    cbrCollector := collector.NewCBRCollector(producer, cfg)
    cryptoCollector := collector.NewCryptoCollector(producer, cfg)

    go func() {
        cbrCollector.Collect()                     // запуск сразу при старте
        t := time.NewTicker(cfg.CBRInterval)       // по умолчанию: 86400с
        for range t.C { cbrCollector.Collect() }
    }()

    go func() {
        cryptoCollector.Collect()                  // запуск сразу при старте
        t := time.NewTicker(cfg.CryptoInterval)   // по умолчанию: 60с
        for range t.C { cryptoCollector.Collect() }
    }()

    select {} // бесконечное ожидание
}
```

**Плюсы:** коллектор масштабируется и перезапускается независимо, не затрагивая API-сервисы.  
**Минусы:** если Kafka недоступна, события теряются (локальной очереди нет).

---

## 13. Развёртывание и инфраструктура

### Монолит (`docker-compose.yml`)

```
Сервисов:       3 контейнера
Инфраструктура: 1× PostgreSQL 14

postgres:                 → currency_db (все таблицы)
currency_tracker_web:     → бинарник cmd/server (порт 8081)
currency_tracker_bot:     → бинарник cmd/bot (без порта)

Порядок запуска:
  postgres (healthy) → web и bot одновременно
```

**Volumes:** `postgres_data`  
**Окружение:** общий файл `.env`, загружается обоими бинарниками  
**Сборка:** один `Dockerfile` на бинарник; нет сложностей с multi-stage

### Микросервисы (`docker-compose.yml`)

```
Инфраструктурные сервисы (7):
  postgres-auth    :5432  → auth_db
  postgres-history :5433  → history_db
  clickhouse       :9000  → default db (crypto_rates)
  redis            :6379  → подписки
  zookeeper        :2181  → координация Kafka
  kafka            :9092  → 2 топика × 3 партиции
  kafka-init              → создаёт топики (one-shot контейнер)

Прикладные сервисы (8):
  auth-service       :8082   depends: postgres-auth (healthy)
  data-collector             depends: kafka (healthy), kafka-init (done)
  normalization-service      depends: kafka (healthy), kafka-init (done)
  history-service    :8084   depends: postgres-history, clickhouse, kafka (all healthy)
  notification-service:8085  depends: redis, kafka (all healthy)
  telegram-bot               depends: api-gateway, notification-service
  api-gateway        :8080   depends: auth, history, notification services
  web-ui             :3000   depends: api-gateway

Итого контейнеров:  15
```

**Volumes:** `postgres_auth_data`, `postgres_history_data`, `clickhouse_data`

**Healthchecks:** у каждого хранилища данных есть Docker-healthcheck → прикладные сервисы стартуют только после того, как зависимости здоровы, что исключает ошибки подключения при холодном старте.

---

## 14. Примеры прохождения запросов

### Пример А: Получить курсы ЦБ РФ за конкретную дату

**Монолит:**
```
GET /rates/cbr?date=2024-01-15
    │
    ├─ парсинг даты → time.Time
    ├─ db.GetCurrencyRatesByDate(2024-01-15)     [PostgreSQL SELECT]
    │       ├─ строки найдены → вернуть немедленно
    │       │
    │       └─ строк нет:
    │           cbr.GetCBRRatesByDate("2024-01-15")
    │               HTTP GET /archive/2024/01/15/daily_json.js  (таймаут 10с)
    │               JSON decode → map[string]Valute
    │           go db.SaveCurrencyRates(rates)   [асинхронно, без блокировки]
    │
    └─ вернуть: {"success":true,"data":{"USD":{...},"EUR":{...}}}
```

**Микросервисы:**
```
GET /rates/cbr?date=2024-01-15
    │
    api-gateway: прокси на history-service
    │
    history-service: GetCBRHistory(date=2024-01-15)
    ├─ db.GetCurrencyRatesByDate(2024-01-15)     [PostgreSQL SELECT]
    │       ├─ строки найдены → вернуть немедленно
    │       │
    │       └─ строк нет:
    │           cbrbackfill.Client.Fetch("2024-01-15")
    │               HTTP GET /archive/2024/01/15/daily_json.js
    │           db.SaveCurrencyRates(rates)       [синхронно, затем вернуть]
    │
    └─ вернуть: [{"id":1,"date":"2024-01-15","currency_code":"USD","value":89.50}]
```

**Отличия:**
- Формат ответа разный: монолит возвращает map в обёртке `{success, data}`, микросервисы — плоский JSON-массив
- Монолит сохраняет асинхронно; history-service сохраняет синхронно (гарантирует фиксацию до ответа клиенту)
- Микросервисы добавляют один HTTP-хоп (gateway → history-service)

---

### Пример Б: Подписаться на оповещения о цене крипто

**Монолит:**
```
Telegram-пользователь: /crypto_subscribe BTC

bot.handleCryptoSubscribe():
    symbol = "BTC"
    mu.Lock()
    cryptoSubs[user.ID] = append(cryptoSubs[user.ID], "BTC")
    mu.Unlock()
    db.SaveCryptoSubscription(user.ID, "BTC")   [INSERT OR IGNORE]
    bot.Send(user, "✅ Вы подписались на BTC!")

→ через 15 минут (тик планировщика):
    binance.GetCurrentCryptoToRubRate("BTC")
    if price != lastCryptoPrices["BTC"]:
        bot.Send(user, "📈 BTC: 4,500,000 RUB")
        lastCryptoPrices["BTC"] = price
```

**Микросервисы:**
```
Telegram-пользователь: /crypto_subscribe BTC

bot.handleCryptoSubscribe():
    symbol = "BTC"
    POST http://notification-service:8085/subscriptions/crypto
         {"telegram_id": 123456789, "value": "BTC"}
    bot.Send(user, "✅ Вы подписались на BTC!")

→ notification-service сохраняет:
    Redis SADD crypto:123456789 "BTC"

→ ~через 60 секунд (тик data-collector):
    data-collector публикует RawCryptoRatesEvent в raw-rates

→ normalization-service:
    потребляет raw-rates
    вычисляет PriceRUB = 42000 USDT × 89.50 RUB/USD = 3,759,000 RUB
    публикует NormalizedCryptoRatesEvent в normalized-rates

→ подписчик notification-service:
    потребляет normalized-rates
    symbol = "BTCUSDT" → убрать суффикс → "BTC"
    SCAN redis: ключи вида crypto:*
    для каждого ключа:
        telegram_id = извлечь из ключа
        members = SMEMBERS crypto:<telegram_id>
        если "BTC" в members:
            Telegram API: отправить "💰 BTC: 3,759,000.00 RUB" пользователю
```

---

### Пример В: Экспорт истории крипто в Excel (только монолит)

```
GET /rates/crypto/history/range/excel?symbol=BTC&from=2024-01-01&to=2024-01-07

GetCryptoHistoryByDateRangeHandler:
    получить курсы (та же логика, что и у /range)
    │
    excelize.NewFile()
    xlsx.SetSheetName("Sheet1", "BTC Rates")
    xlsx.SetCellValue(...) для заголовков (Date, Open, High, Low, Close, Volume)
    стиль: жирные заголовки, числовой формат для цен
    для каждой строки с курсами:
        xlsx.SetCellValue("A"+row, date.Format("2006-01-02"))
        xlsx.SetCellFloat("B"+row, open, ...)
        ...
    w.Header().Set("Content-Disposition", "attachment; filename=BTC_rates.xlsx")
    w.Header().Set("Content-Type", "application/vnd.openxmlformats-...")
    xlsx.Write(w)
```

В реализации на микросервисах этой функции нет.

---

## 15. Обработка ошибок и отказоустойчивость

### Монолит

| Сбой | Влияние | Восстановление |
|---------|--------|----------|
| PostgreSQL недоступен | **Все эндпоинты не работают** | Ручной рестарт; повторных попыток нет |
| ЦБ РФ API недоступен | Возвращает кэшированные данные; если кэш пуст → 500 | Нет |
| Binance API недоступен | Крипто-эндпоинты не работают; 3 попытки с backoff | 3 повтора, затем ошибка |
| Краш сервера | Планировщик сбрасывается; подписки перезагружаются из БД | PostgreSQL — источник истины |
| Краш бота | Уведомления отсутствуют до рестарта | Подписки сохранены в БД |

Асинхронные записи в БД (без блокировки) означают, что краш между ответом API и завершением записи приводит к **незаметной потере данных**.

### Микросервисы

| Сбой | Влияние | Восстановление |
|---------|--------|----------|
| `postgres-auth` недоступен | Вход/регистрация не работают; JWT-валидация падает → все защищённые маршруты 401 | Рестарт контейнера |
| `postgres-history` недоступен | Эндпоинты истории ЦБ РФ не работают; крипто через ClickHouse не затронуто | Рестарт контейнера |
| ClickHouse недоступен | История крипто не работает; история ЦБ РФ не затронута | Рестарт контейнера |
| Redis недоступен | Управление подписками не работает; текущие уведомления теряются | Рестарт контейнера |
| Kafka недоступна | Сбор данных останавливается; подписчики history/notification зависают | Рестарт Kafka; консьюмеры возобновляют с последнего зафиксированного offset |
| Краш `normalization-service` | События накапливаются в Kafka; обрабатываются при рестарте (трекинг offset) | Автовозобновление |
| Краш `history-service` | События `normalized-rates` накапливаются в Kafka; переобрабатываются при рестарте | Идемпотентные upsert предотвращают дубли |
| Краш `api-gateway` | Весь клиентский трафик падает | Рестарт контейнера |

**Kafka consumer group** обеспечивает доставку at-least-once: если `history-service` крашится в процессе обработки, он переобработает последний незафиксированный батч при рестарте. `ReplacingMergeTree` в ClickHouse и `ON CONFLICT DO UPDATE` в PostgreSQL делают повторные воспроизведения безопасными.

---

## 16. Тестирование

### Монолит

Тесты находятся в `monolith/` и покрывают все внутренние пакеты:

| Файл теста | Тип | Что тестирует |
|-----------|------|---------------|
| `internal/api/handlers_test.go` | Интеграционный | HTTP-обработчики с реальным Postgres (TestContainers) |
| `internal/currency/cbr/cbr_test.go` | Юнит | Парсинг XML/JSON ЦБ РФ |
| `internal/currency/binance/binance_test.go` | Юнит | Парсинг klines Binance, конвертация в рубли |
| `internal/scheduler/scheduler_test.go` | Юнит | Логика расписания |
| `internal/storage/postgres_test.go` | Интеграционный | CRUD БД с реальным Postgres (TestContainers) |

Запуск интеграционных тестов: `go test ./...` (требует Docker)  
Пропустить интеграционные тесты: `go test -short ./...`

### Микросервисы

Тесты разбросаны по сервисам, покрытие меньше:

| Файл теста | Тип | Что тестирует |
|-----------|------|---------------|
| `normalization-service/.../normalizer_test.go` | Юнит | Парсинг дат, конвертация крипто в рубли |
| `history-service/.../handler_test.go` | Юнит | Логика обработчиков (мок хранилища) |
| `history-service/.../interval_test.go` | Юнит | Маппинг `IntervalForCalendarSpan()` |
| `history-service/.../types_test.go` | Юнит | Конвертации типов |
| `history-service/.../crypto_fill_test.go` | Юнит | Парсинг запросов бэкфила |
| `history-service/.../client_test.go` | Юнит | HTTP-клиент бэкфила |
| `auth-service/.../handler_test.go` | Юнит | Логика регистрации/входа/выхода |

Интеграционных тестов с Kafka нет. TestContainers не используется. End-to-end тестирование требует `docker-compose up`.

---

## 17. Итоговая сравнительная таблица

| Аспект | Монолит | Микросервисы |
|--------|----------|---------------|
| **Стиль архитектуры** | Слоистый монолит | Event-driven микросервисы |
| **Количество бинарников** | 2 (server, bot) | 8 сервисов |
| **Поток данных** | Синхронный, request-driven | Async (Kafka) + Sync (HTTP) |
| **Базы данных** | 1× PostgreSQL (все данные) | 2× PostgreSQL + ClickHouse + Redis |
| **Брокер сообщений** | Нет | Kafka (2 топика, по 3 партиции) |
| **Аутентификация** | Нет (всё публично) | JWT через auth-service |
| **Сбор курсов** | По требованию + ежедневный планировщик | Непрерывная публикация (Kafka producer) |
| **Хранение крипто** | PostgreSQL (OHLCV в рублях) | ClickHouse (оптимизирован для временных рядов) |
| **Хранение подписок** | PostgreSQL + in-memory map | Redis sets |
| **Триггер уведомлений** | Опрос планировщика (15 мин) | Kafka event push (~60с) |
| **Промах кэша ЦБ РФ** | Асинхронное сохранение (риск потери данных) | Синхронное сохранение |
| **Конвертация крипто в рубли** | В Binance-клиенте (на каждый запрос) | В normalization-service (на каждое событие) |
| **Формат API** | `{"success":true,"data":{...}}` | Плоские JSON-массивы |
| **Экспорт в Excel** | Да (`/range/excel`) | Нет |
| **Изоляция отказов** | Нет — один сбой = полный отказ | Высокая — сервисы падают независимо |
| **Горизонтальное масштабирование** | Load balancer + общая БД | Масштабирование по отдельным сервисам |
| **Сложность холодного старта** | Низкая (2 контейнера) | Высокая (15 контейнеров, упорядоченный запуск) |
| **Хопов на запрос** | 1 (клиент → сервер) | 2–3 (клиент → gateway → сервис → БД) |
| **Покрытие тестами** | Высокое (интеграционные + юнит) | Среднее (только юнит) |
| **Наблюдаемость** | Только plaintext-логи | Логи по сервисам + healthchecks |
| **Распределённая трассировка** | Не применимо | Не реализована |
