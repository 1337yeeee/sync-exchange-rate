# CNB Exchange Rate Synchronization and Reports

Проект реализует систему синхронизации курсов валют Чешского национального банка (CNB) относительно чешской кроны и построения отчетов по выбранным валютам.

Система умеет:

- получать дневные курсы CNB через `daily.txt`;
- получать исторические курсы CNB через `year.txt`;
- сохранять данные в Postgres;
- выполнять ручную синхронизацию за дату или период;
- выполнять автоматическую синхронизацию по расписанию;
- строить JSON-отчет с `min`, `max`, `avg`, `observations`;
- считать показатели для `Amount = 1` через `NormalizedRate`;
- отдавать простой web-интерфейс для демонстрации.

## Стек

- Go 1.25
- PostgreSQL
- GORM
- net/http
- Docker Compose
- HTML/CSS/JS без frontend-фреймворка

## Структура Проекта

```text
.
├── app
│   ├── cmd
│   │   ├── server
│   │   └── scheduler
│   ├── internal
│   │   ├── app
│   │   ├── client/cnb
│   │   ├── config
│   │   ├── delivery/http
│   │   ├── domain
│   │   ├── repository
│   │   ├── scheduler
│   │   ├── service/report
│   │   ├── service/sync
│   │   └── storage/database
│   ├── Dockerfile
│   ├── go.mod
│   └── go.sum
├── docker-compose.yml
└── .env.example
```

## Компоненты

### `cmd/server`

Основной HTTP backend.

Отвечает за:

- загрузку конфигурации;
- подключение к Postgres;
- запуск миграций;
- создание CNB-клиента, репозитория и сервисов;
- запуск HTTP API;
- graceful shutdown по `SIGINT`/`SIGTERM`;
- опциональный embedded scheduler через `SCHEDULER_ENABLED=true`.

По умолчанию embedded scheduler выключен, потому что scheduler вынесен в отдельный автономный процесс.

### `cmd/scheduler`

Автономный runtime для планировщика.

Отвечает за:

- загрузку той же конфигурации, что и backend;
- ожидание готовности Postgres с retry/backoff;
- создание CNB-клиента, репозитория и sync-service;
- запуск scheduler по `SYNC_SCHEDULE`;
- логирование запуска, ошибок и результатов синхронизации;
- graceful shutdown.

Этот процесс может работать независимо от HTTP backend. Если остановить сервис `app`, сервис `scheduler` продолжит работать, пока доступна БД.

### `internal/app`

Общий bootstrap приложения.

Собирает общие зависимости:

- подключение к БД;
- readiness-check Postgres;
- миграции;
- CNB client;
- repository;
- sync-service;
- report-service.

Используется и `cmd/server`, и `cmd/scheduler`, чтобы не дублировать wiring.

### `internal/config`

Загрузка и валидация конфигурации из переменных окружения.

Отвечает за:

- HTTP-порт;
- параметры Postgres;
- расписание синхронизации;
- включение/выключение embedded scheduler;
- список валют;
- период исторической синхронизации;
- базовый URL CNB.

### `internal/client/cnb`

Клиент и парсеры CNB.

HTTP client:

- `FetchDaily(ctx, date)` получает `daily.txt?date=DD.MM.YYYY`;
- `FetchYear(ctx, year)` получает `year.txt?year=YYYY`.

Парсеры:

- `ParseDaily` разбирает дневной формат `Country|Currency|Amount|Code|Rate`;
- `ParseYear` разбирает исторический формат `Date|1 EUR|100 JPY|...`;
- пустые и битые строки пропускаются;
- отсутствующие значения не ломают синхронизацию;
- курсы нормализуются до `Amount = 1`.

### `internal/domain`

Доменная модель `Rate`.

Основные поля:

- `TradingDate`;
- `Country`;
- `CurrencyName`;
- `CurrencyCode`;
- `Amount`;
- `Rate`;
- `NormalizedRate`.

`NormalizedRate` считается как:

```text
NormalizedRate = Rate / Amount
```

Например, если CNB отдает `100 HUF = 8.012 CZK`, то для одной единицы:

```text
1 HUF = 0.08012 CZK
```

### `internal/storage/database`

Подключение к Postgres и миграции через GORM.

Мигрируется таблица `rates`.

Уникальность задается парой:

```text
trading_date + currency_code
```

Это позволяет делать upsert и не создавать дубли при повторной синхронизации.

### `internal/repository`

Интерфейс доступа к курсам:

- `Save`;
- `GetByPeriod`;
- `GetExistingDates`.

### `internal/repository/postgres`

Postgres-реализация репозитория.

Отвечает за:

- upsert курсов;
- выборку за период;
- фильтрацию по валютам;
- получение существующих дат;
- работу с `context.Context`.

### `internal/service/sync`

Сервис синхронизации.

Умеет:

- синхронизировать одну дату через `daily.txt`;
- синхронизировать период через `year.txt`;
- фильтровать валюты из конфига;
- пропускать отсутствующие данные;
- не падать при частичных ошибках CNB;
- считать результат синхронизации.

Результат синхронизации содержит:

```json
{
  "savedCount": 4,
  "skippedCount": 2,
  "errors": []
}
```

Повторная синхронизация идемпотентна: уже существующие неизменившиеся записи считаются skipped.

### `internal/service/report`

Сервис отчетов.

Строит отчет по выбранным валютам за период:

- `minRate`;
- `maxRate`;
- `avgRate`;
- `observations`.

Все значения считаются по `NormalizedRate`, то есть для одной условной единицы валюты.

Если по валюте нет данных, сервис возвращает запись с `observations = 0` и `null` в метриках.

### `internal/delivery/http`

HTTP API и встроенный frontend.

Маршруты:

- `GET /health`;
- `POST /sync`;
- `GET /reports/rates`;
- `GET /` для HTML-интерфейса.

Frontend встроен через `embed` и находится в:

```text
app/internal/delivery/http/static/index.html
```

### `internal/scheduler`

In-process планировщик, который используется отдельным `cmd/scheduler` runtime.

Поддерживает cron-like формат из 5 полей:

```text
minute hour * * *
```

Примеры:

```text
1 0 * * *      каждый день в 00:01
*/15 * * * *   каждые 15 минут
```

Текущие ограничения:

- поле дня месяца должно быть `*`;
- поле месяца должно быть `*`;
- поле дня недели должно быть `*`.

При срабатывании scheduler вызывает `SyncDate` для даты срабатывания.

## Переменные Окружения

Пример находится в `.env.example`.

### HTTP

| Переменная | Значение по умолчанию | Описание |
|---|---:|---|
| `HTTP_PORT` | `8080` | Порт HTTP API и встроенного frontend. |

### Postgres

| Переменная | Значение по умолчанию | Описание |
|---|---:|---|
| `POSTGRES_DSN` | пусто | Полный DSN для подключения к Postgres. Если задан, отдельные поля подключения не используются. |
| `POSTGRES_HOST` | `localhost` | Хост Postgres. В Docker Compose переопределяется на `postgres`. |
| `POSTGRES_PORT` | `5432` | Порт Postgres. |
| `POSTGRES_USER` | `postgres` | Пользователь БД. |
| `POSTGRES_PASSWORD` | `postgres` | Пароль пользователя БД. |
| `POSTGRES_DB` | `exchange_rates` | Имя базы данных. |
| `POSTGRES_SSLMODE` | `disable` | SSL mode для подключения. |
| `POSTGRES_TIMEZONE` | `UTC` | TimeZone для подключения GORM/Postgres. |

### Синхронизация

| Переменная | Значение по умолчанию | Описание |
|---|---:|---|
| `SYNC_SCHEDULE` | `1 0 * * *` | Расписание автоматической синхронизации. |
| `SCHEDULER_ENABLED` | `false` | Включает embedded scheduler внутри HTTP app. Для Docker Compose должен быть `false`, потому что есть отдельный сервис `scheduler`. |
| `SYNC_CURRENCIES` | `USD,EUR,GBP,RUB` | Валюты, которые синхронизируются автоматически и вручную через sync-service. |
| `SYNC_HISTORY_START_DATE` | `2019-01-01` | Начало исторического периода в конфигурации. |
| `SYNC_HISTORY_END_DATE` | `2019-12-31` | Конец исторического периода в конфигурации. |

### CNB

| Переменная | Значение по умолчанию | Описание |
|---|---|---|
| `CNB_BASE_URL` | `https://www.cnb.cz/en/financial_markets/foreign_exchange_market/exchange_rate_fixing` | Базовый URL CNB API. |

## Запуск Через Docker Compose

1. Создать `.env` из примера:

```bash
cp .env.example .env
```

2. Запустить все сервисы:

```bash
docker compose up --build
```

Будут запущены:

- `postgres`;
- `app`;
- `scheduler`.

3. Проверить API:

```bash
curl http://localhost:8080/health
```

Ожидаемый ответ:

```json
{"status":"ok"}
```

4. Открыть frontend:

```text
http://localhost:8080/
```

## Запуск Только Автономного Scheduler

Scheduler может работать без HTTP backend:

```bash
docker compose up --build postgres scheduler
```

Проверить логи:

```bash
docker compose logs scheduler
```

В логах должно быть видно:

- старт scheduler runtime;
- расписание;
- список валют;
- готовность Postgres;
- старт scheduler;
- результаты sync при наступлении расписания.

## Проверка Независимости Scheduler

Можно остановить HTTP app:

```bash
docker compose stop app
```

Проверить, что scheduler продолжает работать:

```bash
docker compose ps
```

Сервис `scheduler` должен оставаться в статусе `Up`.

## Локальный Запуск Без Docker

Нужен запущенный Postgres и переменные окружения.

Из директории `app`:

```bash
go run ./cmd/server
```

Отдельный scheduler:

```bash
go run ./cmd/scheduler
```

Сборка бинарников:

```bash
go build ./cmd/server ./cmd/scheduler
```

## HTTP API

### Health

```http
GET /health
```

Пример:

```bash
curl http://localhost:8080/health
```

Ответ:

```json
{
  "status": "ok"
}
```

### Синхронизация За Дату

```http
POST /sync?startDate=YYYY-MM-DD
```

Пример:

```bash
curl -X POST "http://localhost:8080/sync?startDate=2019-07-26"
```

### Синхронизация За Период

```http
POST /sync?startDate=YYYY-MM-DD&endDate=YYYY-MM-DD
```

Пример:

```bash
curl -X POST "http://localhost:8080/sync?startDate=2019-07-26&endDate=2019-07-28"
```

Ответ:

```json
{
  "savedCount": 4,
  "skippedCount": 0,
  "errors": []
}
```

### Отчет По Курсам

```http
GET /reports/rates?startDate=YYYY-MM-DD&endDate=YYYY-MM-DD&currencies=USD,EUR
```

Пример:

```bash
curl "http://localhost:8080/reports/rates?startDate=2019-07-26&endDate=2019-07-28&currencies=EUR,USD"
```

Ответ:

```json
{
  "startDate": "2019-07-26",
  "endDate": "2019-07-28",
  "currencies": [
    {
      "currencyCode": "EUR",
      "minRate": 25.54,
      "maxRate": 25.64,
      "avgRate": 25.59,
      "observations": 2
    },
    {
      "currencyCode": "USD",
      "minRate": 22.932,
      "maxRate": 23.132,
      "avgRate": 23.032,
      "observations": 2
    }
  ]
}
```

## Web Interface

Frontend доступен по адресу:

```text
http://localhost:8080/
```

В интерфейсе можно:

- указать `startDate`;
- указать `endDate`;
- указать список валют;
- запустить синхронизацию;
- построить отчет;
- увидеть таблицу `currency`, `min`, `max`, `avg`, `observations`.

## Как Работает Синхронизация

Для одной даты используется CNB daily endpoint:

```text
daily.txt?date=DD.MM.YYYY
```

Для периода используется CNB yearly endpoint:

```text
year.txt?year=YYYY
```

Если период захватывает несколько лет, сервис загружает данные по каждому году отдельно.

Данные фильтруются по `SYNC_CURRENCIES`, нормализуются до `Amount = 1` и сохраняются в таблицу `rates`.

При повторной синхронизации:

- если запись уже есть и не изменилась, она считается skipped;
- если запись есть, но изменилась, выполняется update;
- если записи нет, выполняется insert.

## Как Работает Отчет

Report-service получает из репозитория курсы за период и список валют.

Для каждой валюты рассчитываются:

- `minRate`;
- `maxRate`;
- `avgRate`;
- `observations`.

Расчет идет по `NormalizedRate`, а не по исходному `Rate`.

Это важно для валют, у которых CNB публикует курс за 100 единиц, например `HUF` или `JPY`.

## Обработка Ошибок И Пропусков

Система не падает при частично отсутствующих данных CNB.

Примеры:

- если в `year.txt` нет значения по валюте в конкретную дату, оно пропускается;
- если строка в CNB-файле битая, она пропускается;
- если часть годов недоступна, ошибка записывается в `errors`, а доступные годы продолжают обрабатываться;
- если по валюте нет данных в отчете, метрики возвращаются как `null`, а `observations` равен `0`.

## Тесты

Запуск всех тестов:

```bash
cd app
GOCACHE=/private/tmp/go-cache go test ./...
```

Покрыты:

- конфигурация;
- CNB HTTP client;
- daily parser;
- year parser;
- доменная нормализация;
- миграции;
- Postgres repository;
- sync-service;
- report-service;
- HTTP handlers;
- scheduler;
- интеграционные сценарии.

## Docker Команды

Запустить все:

```bash
docker compose up --build
```

Запустить в фоне:

```bash
docker compose up --build -d
```

Остановить:

```bash
docker compose down
```

Остановить вместе с volume БД:

```bash
docker compose down -v
```

Посмотреть логи backend:

```bash
docker compose logs app
```

Посмотреть логи scheduler:

```bash
docker compose logs scheduler
```

Проверить статус:

```bash
docker compose ps
```

## Особенности Реализации

- Scheduler вынесен в отдельный runtime, поэтому может работать без HTTP backend.
- HTTP backend по умолчанию не запускает scheduler, чтобы не было двойной синхронизации.
- Таблица валют отдельно не используется: список валют хранится в конфигурации, а нужные атрибуты валюты сохраняются в `rates`.
- Данные хранятся персистентно в Postgres volume `postgres_data`.
- Frontend встроен в Go binary через `embed`, поэтому отдельная сборка frontend не нужна.
- Docker image содержит оба бинарника: `server` и `scheduler`.

## Быстрый Демонстрационный Сценарий

1. Запустить проект:

```bash
docker compose up --build -d
```

2. Открыть:

```text
http://localhost:8080/
```

3. Ввести:

```text
startDate: 2019-07-26
endDate: 2019-07-28
currencies: EUR,USD
```

4. Нажать `Run sync`.

5. Нажать `Build report`.

6. Проверить таблицу отчета.
