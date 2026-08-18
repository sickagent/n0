# Сервисы и конфигурация

Эта страница описывает runtime-сервисы n0, их зависимости, порты и минимальные
примеры конфигурации. Для локального запуска используйте [`.env.example`](../.env.example)
и `deployments/docker-compose.yml`.

## Состав системы

| Сервис | Назначение | HTTP | gRPC | Метрики |
|---|---|---:|---:|---:|
| `agent-gateway` | Публичный REST/MCP API, JWT, CORS и tenant context | `:8081` | `:8080` | `:9090` |
| `meta-service` | Пользователи, workspaces, connections, схема и плагины | `:8081` | `:8080` | `:9090` |
| `query-engine` | Жизненный цикл jobs, sandbox SQL и workers | `:8082` | `:8080` | `:9090` |
| `connection-manager` | Драйверы баз, schema discovery и выполнение SQL | `:8082` | `:8080` | `:9090` |
| `web-admin` | React-интерфейс администратора | `:80` | — | — |

В Docker Compose одинаковые внутренние порты изолированы сетью `n0-net`; наружу
они опубликованы на разные порты. Внутренние сервисы не следует публиковать во
внешний ingress.

### Зависимости

```text
web-admin -> agent-gateway -> meta-service -> PostgreSQL
                         -> query-engine -> Redis + S3-compatible storage
                         -> connection-manager -> целевые базы данных

Все Go-сервисы -> NATS Core + JetStream
```

- `meta-service` владеет metadata PostgreSQL и создаёт streams `AUDIT` и `QUERIES`.
- `query-engine` использует NATS для jobs, Redis для состояния и небольших
  результатов, S3-compatible storage для больших результатов.
- `connection-manager` не хранит connection config: он получает расшифрованные
  параметры только на время операции.
- Сервисы находят друг друга через NATS request/reply; `*_ADDR` используются как
  fallback-адреса.

## Общие правила конфигурации

Конфигурация читается из flags и environment variables. Для каждого имени
поддерживается вариант `N0_<NAME>`; он имеет приоритет над обычным именем.
Например, `N0_NATS_URL` перекрывает `NATS_URL`.

Адреса зависят от места запуска:

| Где запущены сервисы | Адрес NATS | Адрес PostgreSQL | Адреса сервисов |
|---|---|---|---|
| Docker Compose | `nats:4222` | `postgres:5432` | имена контейнеров, например `meta-service:8080` |
| локально на host | `localhost:4222` | `localhost:5432` | `localhost:<published-port>` |

В production обязательны непредсказуемые `JWT_SECRET` и `ENCRYPTION_KEY`.
Оба значения — Base64; после декодирования ключ должен быть не короче 32 байт.
Не храните production-секреты в `.env` и не используйте ключи из примеров.

## Пример общего `.env`

```dotenv
ENVIRONMENT=development
LOG_LEVEL=info
NATS_URL=nats://nats:4222

# Meta Service
POSTGRES_DSN=postgres://postgres:postgres@postgres:5432/meta?sslmode=disable
CONNECTION_MANAGER_ADDR=connection-manager:8080
ENCRYPTION_KEY=bjAtZGV2ZWxvcG1lbnQtYWVzLWtleS0zMi1ieXRlcyE=

# Agent Gateway
META_SERVICE_ADDR=meta-service:8080
META_SERVICE_HTTP_URL=http://meta-service:8081
QUERY_ENGINE_ADDR=query-engine:8080
JWT_SECRET=bjAtZGV2ZWxvcG1lbnQtand0LWtleS0zMi1ieXRlcyE=
JWT_EXPIRY_HOURS=8
CORS_ALLOWED_ORIGINS=http://localhost:3000

# Query Engine
WORKER_COUNT=4
REDIS_MODE=standalone
REDIS_ADDR=redis:6379
JOB_TTL_HOURS=24
S3_ENDPOINT=minio:9000
S3_ACCESS_KEY=n0
S3_SECRET_KEY=n0-development-secret
S3_BUCKET=n0-results
S3_USE_SSL=false
RESULT_INLINE_MAX_BYTES=1048576
```

Этот файл предназначен для Docker Compose. При запуске бинарников напрямую
замените имена контейнеров на `localhost` и опубликованные порты.

## Конфигурация сервисов

### Agent Gateway

Публичный вход для Web Admin, внешних агентов и MCP. Минимальный набор:

```dotenv
APP_NAME=agent-gateway
ENVIRONMENT=production
LOG_LEVEL=info
NATS_URL=nats://nats.internal:4222
GRPC_ADDR=:8080
HTTP_ADDR=:8081
META_SERVICE_ADDR=meta-service:8080
META_SERVICE_HTTP_URL=http://meta-service:8081
QUERY_ENGINE_ADDR=query-engine:8080
CONNECTION_MANAGER_ADDR=connection-manager:8080
JWT_SECRET=<base64-encoded-random-key-at-least-32-bytes>
JWT_EXPIRY_HOURS=8
CORS_ALLOWED_ORIGINS=https://admin.example.com
```

Публичные endpoints: `GET /health`, `/v1/...` и `/mcp`. В production оставьте
наружу только HTTP-порт Gateway; его gRPC-порт нужен для внутреннего трафика.

### Meta Service

Хранит пользователей, workspaces, connections, snapshots схемы, плагины и аудит.

```dotenv
APP_NAME=meta-service
ENVIRONMENT=production
LOG_LEVEL=info
NATS_URL=nats://nats.internal:4222
GRPC_ADDR=:8080
GRPC_ADVERTISE_ADDR=meta-service:8080
HTTP_ADDR=:8081
POSTGRES_DSN=postgres://n0_meta:<password>@postgres.internal:5432/n0_meta?sslmode=require
CONNECTION_MANAGER_ADDR=connection-manager:8080
ENCRYPTION_KEY=<base64-encoded-random-32-byte-key>
```

Health check: `GET /healthz`. Перед запуском примените миграции командой
`make migrate-up`. В production без `ENCRYPTION_KEY` сервис завершает запуск.

### Query Engine

Принимает job через внутренний API, запускает worker pool и сохраняет результат.

```dotenv
APP_NAME=query-engine
ENVIRONMENT=production
LOG_LEVEL=info
NATS_URL=nats://nats.internal:4222
GRPC_ADDR=:8080
GRPC_ADVERTISE_ADDR=query-engine:8080
HTTP_ADDR=:8082
META_SERVICE_ADDR=meta-service:8080
CONNECTION_MANAGER_ADDR=connection-manager:8080
WORKER_COUNT=8
REDIS_MODE=standalone
REDIS_ADDR=redis.internal:6379
REDIS_USERNAME=n0
REDIS_PASSWORD=<redis-password>
REDIS_DB=0
JOB_TTL_HOURS=24
S3_ENDPOINT=s3.example.com
S3_ACCESS_KEY=<access-key>
S3_SECRET_KEY=<secret-key>
S3_BUCKET=n0-results
S3_USE_SSL=true
RESULT_INLINE_MAX_BYTES=1048576
```

Для Redis Cluster вместо standalone используйте:

```dotenv
REDIS_MODE=cluster
REDIS_ADDRS=redis-0:6379,redis-1:6379,redis-2:6379
REDIS_DB=0
```

Health check: `GET /healthz`. В production отсутствие Redis приводит к отказу
запуска; если результат больше `RESULT_INLINE_MAX_BYTES`, без S3 он не должен
сохраняться.

### Connection Manager

Изолирует database drivers и обслуживает встроенные адаптеры `postgres`, `mysql`,
`clickhouse`, `sqlite`, `mssql` и `bigquery`.

```dotenv
APP_NAME=connection-manager
ENVIRONMENT=production
LOG_LEVEL=info
NATS_URL=nats://nats.internal:4222
GRPC_ADDR=:8080
GRPC_ADVERTISE_ADDR=connection-manager:8080
HTTP_ADDR=:8082
VAULT_ADDR=https://vault.internal:8200
```

`VAULT_ADDR` пока зарезервирован для будущего runtime lease flow и сам по себе
не включает получение секретов из Vault. Health check: `GET /healthz`.

### Web Admin

Переменные Vite задаются до `npm run build`:

```dotenv
VITE_API_BASE_URL=https://api.example.com
VITE_CM_API_BASE_URL=https://connection-manager.example.com
```

Обычно `VITE_API_BASE_URL` указывает на Agent Gateway. Connection Manager не
нужно делать публичным без отдельного reverse proxy и политики доступа.

## Примеры параметров подключений

Эти JSON-фрагменты передаются в `params` при `POST /v1/connections`. Пароли
шифруются Meta Service перед сохранением и не возвращаются в API-ответах.

### PostgreSQL

```json
{
  "adapter_type": "postgres",
  "params": {
    "host": "analytics-db.internal",
    "port": "5432",
    "user": "n0_reader",
    "password": "<read-only-password>",
    "database": "analytics",
    "sslmode": "require"
  }
}
```

### MySQL

```json
{
  "adapter_type": "mysql",
  "params": {
    "host": "mysql.internal",
    "port": "3306",
    "user": "n0_reader",
    "password": "<read-only-password>",
    "database": "analytics"
  }
}
```

### SQLite

```json
{
  "adapter_type": "sqlite",
  "params": {
    "path": "/data/analytics.sqlite"
  }
}
```

Для каждого connection задавайте read-only credentials и, если нужна изоляция
арендаторов, policy вида:

```json
{
  "query_policy": {
    "allowed_tables": ["public.orders", "public.customers"],
    "tenant_column": "tenant_id"
  }
}
```

## Проверка после запуска

```bash
docker compose -f deployments/docker-compose.yml ps
curl -fsS http://localhost:8083/health
curl -fsS http://localhost:8085/healthz
curl -fsS http://localhost:8087/healthz
curl -fsS http://localhost:8086/healthz
```

Порты `8085`, `8087` и `8086` — опубликованные HTTP-порты Meta Service,
Query Engine и Connection Manager соответственно; внутри Docker-сети
используются их внутренние HTTP-порты из таблицы выше.
