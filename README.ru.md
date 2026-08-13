# n0

**Русский** · [English](./README.md)

[![CI](https://github.com/sickagent/n0/actions/workflows/ci.yml/badge.svg)](https://github.com/sickagent/n0/actions/workflows/ci.yml)
[![Container images](https://github.com/sickagent/n0/actions/workflows/docker.yml/badge.svg)](https://github.com/sickagent/n0/actions/workflows/docker.yml)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev/)

**Данные становятся инструментом агента. Без прямого доступа к вашим базам.**

AI-BI платформа на Go для безопасного подключения AI-агентов к корпоративным данным. Агент изучает доступные схемы, отправляет аналитический SQL как асинхронную задачу и получает структурированный результат, пригодный для анализа, графиков и отчётов.

`n0` не пытается быть ещё одним чат-ботом или BI-конструктором. Это исполнительный и мета-слой между агентами и источниками данных: он отвечает за подключение, изоляцию арендаторов, проверку запросов, выполнение, жизненный цикл задач и выдачу результатов.

> Проект находится в активной разработке. Основной end-to-end сценарий, policy-enforced выполнение запросов, durable result storage, health routing плагинов и сохранение аудита уже работают. MCP, Vault, RLS для metadata-таблиц и Kubernetes deployment остаются в roadmap.

[Возможности](#возможности) · [Быстрый запуск](#быстрый-запуск) · [Первый API-запрос](#первый-запрос-через-api) · [Архитектура](#как-выполняется-запрос) · [Безопасность](#production-security) · [Разработка](#разработка) · [Roadmap](#roadmap)

## Что такое n0?

Обычный AI-агент умеет написать SQL, но не должен:

- знать постоянные пароли от production-баз;
- подключаться к источникам напрямую;
- выполнять произвольный DDL или DML;
- видеть данные другого арендатора;
- хранить результаты и состояние задач локально;
- самостоятельно решать вопросы retries, timeouts и аудита.

`n0` помещает между агентом и данными контролируемый execution layer:

```text
Agent / Admin UI
        │
        │ HTTPS / REST
        ▼
┌───────────────────┐
│   Agent Gateway   │  authentication, tenant context, public API
└─────────┬─────────┘
          │
          ├──────────────► Meta Service ─────► PostgreSQL
          │                 catalog, users,     metadata
          │                 connections
          │
          └──────────────► Query Engine ─────► JetStream jobs
                            sandbox, workers
                                  │
                                  ▼
                         Connection Manager
                                  │
                                  ▼
                       PostgreSQL / MySQL /
                    ClickHouse / SQLite / ...
```

Агент решает, **что спросить**. Платформа контролирует, **кто имеет доступ, какой запрос допустим и как он будет исполнен**.

## Зачем n0?

Большинство AI-to-data интеграций начинается с передачи агенту connection string и заканчивается набором скриптов без общей модели безопасности. Такой подход быстро ломается при появлении нескольких пользователей, источников и сред.

`n0` строится вокруг четырёх принципов:

1. **Zero trust к агенту.** Агент не получает прямой доступ к базе и не считается доверенной стороной.
2. **Асинхронное выполнение.** Аналитический запрос — это job с отдельными статусом и результатом, а не долгий HTTP-вызов.
3. **Tenant isolation.** Владение connection и job проверяется на каждой публичной операции.
4. **Расширяемая платформа.** Новые источники и capabilities должны подключаться через адаптеры и плагины, а не через изменения core-кода.

Подробное архитектурное обоснование находится в [ADR-001-n0.md](./ADR-001-n0.md).

## Для каких сценариев

- **AI data analyst.** Корпоративный чат или агент изучает схему и отвечает на вопросы по данным без прямого connection string.
- **Embedded analytics.** Продукт отправляет аналитические задачи через стабильный API и получает структурированные строки для собственного UI.
- **Self-service data access.** Команды регистрируют разрешённые источники и выполняют ограниченные read-only запросы через единый gateway.
- **Agent platform.** Внутренняя платформа подключает несколько типов агентов к общему каталогу, очереди задач и политике доступа.
- **Adapter ecosystem.** Команда добавляет специфичный DWH или корпоративный источник, не раскрывая его детали всем потребителям.

## Чем n0 не является

- Это не LLM и не система генерации SQL: модель и агент остаются внешними.
- Это не полноценная замена BI frontend: графики и dashboards могут строиться клиентом поверх результата.
- Это не database proxy общего назначения: публичный путь предназначен для контролируемых аналитических задач.
- Это пока не готовый managed cloud service: production infrastructure разворачивает и обслуживает владелец системы.

## Возможности

- **Каталог подключений** — создание, просмотр и удаление tenant-scoped подключений.
- **Schema discovery** — получение таблиц и колонок через единый API и сохранение schema snapshots.
- **Асинхронные SQL jobs** — submit, status polling и постраничное получение результата.
- **Query guardrails** — только одиночные `SELECT`, ограничение количества строк и запрет опасных операций.
- **Built-in adapters** — PostgreSQL, MySQL, ClickHouse, SQLite, Microsoft SQL Server и BigQuery.
- **Web Admin** — управление аккаунтом, workspaces, connections, plugins и Query Lab.
- **JWT authentication** — отдельные user/agent claims, проверка issuer, audience, expiration и алгоритма подписи.
- **Шифрование connection config** — AES-256-GCM для сохранённых параметров подключения.
- **NATS JetStream workers** — durable consumer, explicit acknowledgements, ограниченные повторные доставки.
- **Plugin registry foundation** — регистрация внешних адаптеров и agent capabilities.
- **Production lifecycle** — bounded HTTP timeouts, request-size limits, graceful shutdown, NATS reconnect/drain.
- **Observability baseline** — структурированные логи, `/metrics` и health endpoints.

## Текущий статус

README намеренно разделяет работающие функции и целевую архитектуру.

| Область | Статус | Что уже есть |
|---|---:|---|
| REST API и Web Admin | ✅ Работает | Auth, connections, schemas, plugins, Query Lab |
| Асинхронные query jobs | ✅ Работает | JetStream submit, workers, polling, pagination |
| Tenant isolation | ✅ Базовый контур | Connections и jobs проверяются по `tenant_id` |
| Built-in DB adapters | ✅ Работает | PostgreSQL, MySQL, ClickHouse, SQLite, MSSQL, BigQuery |
| Credential protection | ✅ Базовый контур | AES-256-GCM at rest, redaction на публичной границе |
| Query sandbox | ✅ Реализовано | Структурный SELECT parser, table allowlist, tenant predicate injection, контроль statements и LIMIT |
| Plugin platform | ✅ Реализовано | Валидация регистрации, persistent lifecycle, gRPC health probes, автоматическое добавление/удаление routes |
| Result persistence | ✅ Реализовано | Redis для job metadata и малых результатов; S3-compatible storage для больших payload с retention |
| Audit pipeline | ✅ Реализовано | JetStream producer ack, durable consumer, explicit ack и идемпотентный PostgreSQL sink |
| MCP server | ⏳ Roadmap | Предусмотрен архитектурой, но не подключён к gateway |
| Vault integration | ⏳ Roadmap | Конфигурация подготовлена, runtime lease flow ещё не реализован |
| Kubernetes / HA | ⏳ Roadmap | Нужны manifests, HPA, PDB, mTLS и production NATS topology |

## Быстрый запуск

### Требования

- Docker Engine с Docker Compose v2;
- Go 1.26+ — только если сервисы запускаются вне Docker;
- Node.js 22+ — только для локальной разработки Web Admin;
- `make`, `curl`; `jq` удобен для примеров ниже.

### 1. Подготовьте конфигурацию

```bash
cp .env.example .env
```

Значения в `.env.example` предназначены исключительно для локальной среды. Не используйте development JWT/AES ключи и пароли в production.

### 2. Запустите инфраструктуру и сервисы

```bash
make up
make migrate-up
```

Проверьте контейнеры:

```bash
docker compose -f deployments/docker-compose.yml ps
```

### 3. Откройте приложение

- Web Admin: [http://localhost:3000](http://localhost:3000)
- Agent Gateway REST API: [http://localhost:8083](http://localhost:8083)
- NATS monitoring: [http://localhost:8222](http://localhost:8222)

Создайте пользователя с паролем длиной от 12 до 64 символов. После входа платформа автоматически создаст `Default Workspace`.

Остановить локальную среду:

```bash
make down
```

## Первый запрос через API

Ниже показан полный путь от регистрации до результата. Все публичные вызовы идут через Agent Gateway.

### 1. Зарегистрируйтесь и войдите

```bash
curl -sS -X POST http://localhost:8083/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"analyst@example.com","password":"change-me-123"}'

TOKEN=$(curl -sS -X POST http://localhost:8083/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"analyst@example.com","password":"change-me-123"}' \
  | jq -r '.token')
```

Публичная регистрация всегда создаёт пользователя с минимальной ролью `user`. Повышение роли должно выполняться отдельным административным процессом.

### 2. Получите workspace

```bash
WORKSPACE_ID=$(curl -sS http://localhost:8083/v1/workspaces \
  -H "Authorization: Bearer $TOKEN" \
  | jq -r '.workspaces[0].id')
```

### 3. Создайте connection

Этот пример подключается к PostgreSQL из локального Docker Compose:

```bash
CONNECTION_ID=$(curl -sS -X POST http://localhost:8083/v1/connections \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{
    \"workspace_id\": \"$WORKSPACE_ID\",
    \"name\": \"Local PostgreSQL\",
    \"adapter_type\": \"postgres\",
    \"params\": {
      \"host\": \"postgres\",
      \"port\": \"5432\",
      \"user\": \"postgres\",
      \"password\": \"postgres\",
      \"database\": \"meta\",
      \"sslmode\": \"disable\"
    }
  }" | jq -r '.connection.id')
```

Параметры подключения шифруются перед сохранением и не возвращаются клиенту при последующем чтении connection.

### 4. Изучите схему

```bash
curl -sS "http://localhost:8083/v1/schema?connection_id=$CONNECTION_ID" \
  -H "Authorization: Bearer $TOKEN" | jq
```

### 5. Отправьте асинхронный запрос

SQL передаётся в JSON body, а не в query string, чтобы текст запроса не попадал в URL, access logs и browser history.

```bash
JOB_ID=$(curl -sS -X POST http://localhost:8083/v1/query \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"connection_id\":\"$CONNECTION_ID\",\"sql\":\"SELECT now() AS server_time\"}" \
  | jq -r '.job_id')
```

### 6. Получите статус и результат

```bash
curl -sS "http://localhost:8083/v1/query/status?job_id=$JOB_ID" \
  -H "Authorization: Bearer $TOKEN" | jq

curl -sS "http://localhost:8083/v1/query/result?job_id=$JOB_ID&page=1&page_size=100" \
  -H "Authorization: Bearer $TOKEN" | jq
```

Gateway самостоятельно добавляет tenant context из JWT. Переданный пользователем `tenant_id` не используется как источник авторизации.

## Как выполняется запрос

```mermaid
sequenceDiagram
    autonumber
    participant A as Agent / Web Admin
    participant G as Agent Gateway
    participant M as Meta Service
    participant N as NATS JetStream
    participant Q as Query Engine
    participant C as Connection Manager
    participant D as Source Database

    A->>G: POST /v1/query
    G->>Q: SubmitQuery(tenant, connection, SQL)
    Q->>N: Publish query job
    Q-->>G: job_id + pending
    G-->>A: Async response with job_id
    N->>Q: Deliver job to worker
    Q->>Q: Validate SQL and enforce row limit
    Q->>M: Get tenant-scoped connection
    M-->>Q: Adapter type + decrypted runtime config
    Q->>C: ExecuteQuery(timeout, limit)
    C->>D: Read-only query
    D-->>C: Rows
    C-->>Q: Structured result
    Q->>Q: Persist job state
    A->>G: GET status / result
    G->>Q: Tenant-scoped lookup
    Q-->>A: Status or result page
```

Сообщение подтверждается worker-ом только после обработки. Poison payload завершается через terminal acknowledgement, а временная ошибка получает ограниченное количество повторных доставок.

## Компоненты

| Компонент | Назначение | Технологии |
|---|---|---|
| **Agent Gateway** | Публичная trust boundary: JWT, CORS, tenant context, REST orchestration | Go, Chi |
| **Meta Service** | Users, agents, workspaces, connections, schema snapshots, plugin registry | Go, PostgreSQL |
| **Query Engine** | Job lifecycle, SQL guardrails, workers, result pagination | Go, NATS JetStream |
| **Connection Manager** | Выбор адаптера, проверка соединения, schema discovery, выполнение | Go, database drivers |
| **Web Admin** | Управление платформой и Query Lab | React, TypeScript, Vite, Mantine |
| **NATS** | Messaging, JetStream queues, discovery и coordination foundation | NATS Core + JetStream |
| **PostgreSQL** | Метаданные платформы | PostgreSQL 16 в dev compose |

Во всех persistent-таблицах платформы используется `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`. Естественные ключи join-таблиц сохраняются отдельными ограничениями `UNIQUE`: идентификаторы остаются единообразными без потери domain-level дедупликации.

### Почему несколько сервисов?

- Gateway можно масштабировать по входящему RPS независимо от query workers.
- Connection Manager изолирует драйверы и секреты от публичного API.
- Query Engine может масштабироваться по глубине очереди.
- Meta Service остаётся единственной точкой владения каталогом и tenancy metadata.

Внутренние сервисы не должны публиковаться во внешний ingress. В production доступ к ним должен ограничиваться network policy и service-to-service identity.

## Адаптеры данных

| Adapter type | Состояние | Основные параметры |
|---|---:|---|
| `postgres` | Built-in | host, port, user, password, database, sslmode |
| `mysql` | Built-in | host, port, user, password, database |
| `clickhouse` | Built-in | host, port, user, password, database |
| `sqlite` | Built-in | path |
| `mssql` | Built-in | host, port, user, password, database |
| `bigquery` | Built-in | project_id, location и provider credentials |

Registry адаптеров находится в [services/connection-manager/internal/registry](./services/connection-manager/internal/registry). Контракты внешних плагинов описаны в [proto/n0/platform/v1/plugin.proto](./proto/n0/platform/v1/plugin.proto).

Для внешних database adapters реализован persistent lifecycle: при регистрации проверяются contract и endpoint, Meta Service каждые 15 секунд вызывает стандартный gRPC Health service, а три последовательных сбоя переводят плагин в `degraded`. Routes со статусом `active` рассылаются в Connection Manager; degraded и disabled routes удаляются немедленно. После рестарта Connection Manager маршруты восстанавливаются очередным health cycle. Внешний плагин обязан реализовать стандартный `grpc.health.v1.Health` вместе с контрактом n0.

## Надёжность результатов и аудита

Query jobs, их состояния и небольшие результаты сохраняются в Redis с настроенным TTL. Результаты больше `RESULT_INLINE_MAX_BYTES` записываются в prefix `results/` настроенного S3-compatible bucket; при запуске устанавливается соответствующая expiration policy. В production сервис не запускается без Redis, а слишком большой результат fail-closed, если object storage не настроен.

Каждое завершённое выполнение запроса публикует tenant-partitioned событие в `audit.events.{tenant}` через JetStream acknowledged publish. Meta Service читает поток durable consumer `postgres-audit-sink`, вставляет событие со стабильным ID в `audit_events` и подтверждает сообщение только после успешной транзакции. Повторная доставка идемпотентна благодаря primary key таблицы.

## Query guardrails

Текущая sandbox-реализация:

- принимает только запрос, начинающийся с `SELECT`;
- запрещает несколько SQL statements;
- запрещает DDL и DML: `CREATE`, `ALTER`, `DROP`, `INSERT`, `UPDATE`, `DELETE`, `TRUNCATE`;
- блокирует `COPY`, `CALL`, `EXECUTE`, `MERGE`, `SELECT INTO` и locking reads;
- добавляет `LIMIT 10000`, если limit отсутствует;
- отклоняет нечисловой или превышающий максимум `LIMIT`;
- передаёт execution timeout в database driver.

Query Engine разбирает консервативное аналитическое подмножество SELECT, извлекает все базовые таблицы, проверяет их по `query_policy.allowed_tables`, отклоняет неоднозначные конструкции и ограничивает `LIMIT`. Если задан `query_policy.tenant_column`, tenant-предикат внедряется с сохранением приоритета булевых выражений. Неподдерживаемый SQL отклоняется. Учётная запись базы всё равно должна быть read-only — это независимый уровень защиты.

Пример политики соединения:

```json
{
  "query_policy": {
    "allowed_tables": ["public.orders", "public.customers"],
    "tenant_column": "tenant_id"
  }
}
```

## Конфигурация

Конфигурация задаётся flags или environment variables. Поддерживаются обычные имена и namespaced-вариант `N0_*`; namespaced значение имеет приоритет.

| Переменная | Назначение | Development default |
|---|---|---|
| `ENVIRONMENT` | `development` или `production` | `development` |
| `LOG_LEVEL` | debug, info, warn, error | `info` |
| `NATS_URL` | Адрес NATS | `nats://localhost:4222` |
| `POSTGRES_DSN` | Metadata PostgreSQL DSN | local PostgreSQL |
| `JWT_SECRET` | Base64 HMAC key, минимум 32 decoded bytes | dev key в `.env.example` |
| `JWT_EXPIRY_HOURS` | Срок действия access token | `24` |
| `ENCRYPTION_KEY` | Base64 AES-256 key для connection config | dev key в `.env.example` |
| `CORS_ALLOWED_ORIGINS` | Разрешённые browser origins через запятую | `http://localhost:3000` |
| `META_SERVICE_ADDR` | Meta Service gRPC endpoint | `localhost:8080` |
| `META_SERVICE_HTTP_URL` | Internal Meta HTTP base URL | `http://localhost:8081` |
| `QUERY_ENGINE_ADDR` | Query Engine gRPC endpoint | `localhost:8082` |
| `CONNECTION_MANAGER_ADDR` | Connection Manager gRPC endpoint | `localhost:8081` |
| `WORKER_COUNT` | Количество query workers | `4` |
| `REDIS_MODE` | Топология Redis: `standalone` или `cluster` | `standalone` |
| `REDIS_ADDR` | Адрес standalone Redis (также обратно совместимый seed кластера) | `localhost:6379` |
| `REDIS_ADDRS` | Адреса seed-узлов Redis Cluster через запятую; имеет приоритет над `REDIS_ADDR` | пусто |
| `REDIS_USERNAME` | Необязательное имя пользователя Redis ACL | пусто |
| `REDIS_PASSWORD` | Необязательный пароль Redis | пусто |
| `REDIS_DB` | База Redis; в cluster mode должна быть `0` | `0` |
| `JOB_TTL_HOURS` | Retention jobs/results | `24` |
| `S3_ENDPOINT` | S3-compatible endpoint для больших результатов | пусто (выключено) |
| `S3_BUCKET` | Bucket больших результатов | `n0-results` |
| `RESULT_INLINE_MAX_BYTES` | Порог переноса из Redis в object storage | `1048576` |
| `VAULT_ADDR` | Зарезервировано для Vault integration | `http://localhost:8200` |

Для Redis Cluster задайте, например, `REDIS_MODE=cluster` и
`REDIS_ADDRS=redis-0:6379,redis-1:6379,redis-2:6379`. Это seed-узлы;
остальную топологию кластера клиент обнаружит автоматически.

Некорректные integer и boolean environment values приводят к startup error, а не молча игнорируются.

## Production security

При `ENVIRONMENT=production`:

- Agent Gateway не запускается без корректного `JWT_SECRET`;
- Meta Service не запускается без `ENCRYPTION_KEY`;
- JWT secret должен декодироваться минимум в 32 байта;
- encryption key должен быть ровно 32 байта для AES-256-GCM.

Перед production deployment также необходимо:

- хранить ключи в Kubernetes Secrets, Vault или cloud secret manager;
- использовать разные JWT и encryption keys;
- включить TLS на ingress, NATS и PostgreSQL;
- изолировать внутренние API network policies;
- включить NATS Accounts/ACL и отдельные tenant subjects;
- создать read-only роли в каждой source database;
- включить PostgreSQL RLS для metadata tables;
- настроить ёмкость и retention JetStream, PostgreSQL audit, Redis и object storage под нагрузку;
- настроить backup, restore и key rotation procedures;
- добавить rate limiting и quota storage во внешнем KV;
- отключить или заменить development credentials из compose.

Docker Compose в этом репозитории — интеграционная среда для разработки, а не production topology.

## API overview

### Public auth

| Method | Path | Назначение |
|---|---|---|
| `POST` | `/v1/auth/register` | Создать пользователя |
| `POST` | `/v1/auth/login` | Получить JWT |
| `GET` | `/v1/auth/me` | Текущий пользователь |

### Connections and catalog

| Method | Path | Назначение |
|---|---|---|
| `GET` | `/v1/workspaces` | Доступные workspaces |
| `GET` | `/v1/connections` | Список connections |
| `POST` | `/v1/connections` | Создать connection |
| `GET` | `/v1/connections/{id}` | Получить connection без credentials |
| `DELETE` | `/v1/connections/{id}` | Удалить принадлежащий tenant connection |
| `POST` | `/v1/test-connection` | Проверить параметры до сохранения |
| `GET` | `/v1/schema?connection_id=...` | Получить schema snapshot |

### Query jobs

| Method | Path | Назначение |
|---|---|---|
| `POST` | `/v1/query` | Создать асинхронную задачу |
| `GET` | `/v1/query/status?job_id=...` | Получить tenant-scoped статус |
| `GET` | `/v1/query/result?job_id=...` | Получить tenant-scoped страницу результата |

### Agents and plugins

| Method | Path | Назначение |
|---|---|---|
| `GET` | `/v1/agents` | Список агентов пользователя |
| `POST` | `/v1/agents` | Зарегистрировать агента |
| `POST` | `/v1/agents/{id}/token` | Выпустить agent token |
| `POST` | `/v1/plugins/register` | Зарегистрировать plugin definition |

Все endpoint-ы, кроме регистрации, входа и health checks, требуют `Authorization: Bearer <token>` в production-конфигурации.

## Разработка

### Структура репозитория

```text
n0/
├── ADR-001-n0.md
├── README.md               # документация на английском
├── README.ru.md            # документация на русском
├── deployments/
│   └── docker-compose.yml
├── pkg/shared/
│   ├── config/            # flags + environment loading
│   ├── crypto/            # AES-256-GCM
│   ├── discovery/         # NATS service discovery
│   ├── graceful/          # shutdown contexts
│   ├── httpserver/        # hardened HTTP defaults
│   ├── jwt/               # user and agent tokens
│   ├── natsclient/        # Core, JetStream and KV client
│   └── observability/     # metrics and health
├── proto/
│   ├── n0/platform/v1/    # source protobuf contracts
│   └── gen/go/            # committed generated Go module
├── services/
│   ├── agent-gateway/
│   ├── connection-manager/
│   ├── meta-service/
│   ├── query-engine/
│   └── web-admin/
└── tests/e2e/
```

### Основные команды

| Команда | Что делает |
|---|---|
| `make up` | Собирает и запускает локальный stack |
| `make down` | Останавливает stack |
| `make migrate-up` | Применяет metadata migrations |
| `make proto` | Генерирует Go protobuf contracts |
| `make build` | Собирает все Go services в `bin/` |
| `make docker-build` | Собирает service images |
| `make test` | Запускает тесты всех Go modules |
| `make test-race` | Запускает Go race detector |
| `make lint` | Запускает golangci-lint для всех Go modules |
| `make tidy` | Синхронизирует Go workspace dependencies |

### Запуск отдельного сервиса

```bash
cd services/meta-service
go run ./cmd \
  --postgres_dsn 'postgres://postgres:postgres@localhost:5432/meta?sslmode=disable' \
  --nats_url 'nats://localhost:4222'
```

### Web Admin

```bash
cd services/web-admin
npm ci
npm run dev
```

Проверка production bundle:

```bash
npm run lint
npm run build
```

## Тестирование и CI

CI выполняет:

- детерминированную генерацию protobuf и проверку drift;
- unit tests всех Go modules;
- Go race detector;
- `go vet`;
- ESLint для Web Admin;
- TypeScript и Vite production build.

Container workflow собирает каждый сервис для `linux/amd64` и `linux/arm64`.
В pull request выполняется только сборка; push в `main` и version tags публикуют
образы вида `ghcr.io/sickagent/n0-agent-gateway` в GitHub Container Registry.
Dependabot отслеживает зависимости Go, npm, Docker и GitHub Actions.

Локально основной набор проверок:

```bash
make test
make test-race

cd services/web-admin
npm run lint
npm run build
```

E2E-набор расположен в [tests/e2e](./tests/e2e) и требует запущенного Docker Compose stack.

## Roadmap

### Security and tenancy

- PostgreSQL Row-Level Security для metadata;
- расширить поддержку SQL-диалектов за пределы консервативного переносимого SELECT subset;
- mTLS/service identity между внутренними сервисами;
- agent ownership verification и token revocation lifecycle;
- distributed rate limiting и quotas через NATS KV или Redis.

### Reliability and scale

- compression результатов, storage-class transitions, replication и signed download URLs;
- idempotency keys и deduplication по `job_id`;
- retry/DLQ topology согласно ADR;
- Kubernetes deployments, HPA, PDB и readiness probes;
- production NATS cluster с replication и accounts.

### Agent platform

- MCP server в Agent Gateway;
- внешний gRPC API для enterprise agents;
- dynamic capability discovery;
- полный plugin lifecycle: validation, health, degraded, deprecated, revoked;
- query transformer hooks и isolated WASM runtime.

### Governance and observability

- audit archival/export integrations и настраиваемые retention policies;
- OpenTelemetry traces между gateway, queue и worker;
- per-tenant usage metrics и budgets;
- credential rotation через Vault leases;
- operational dashboards и SLO alerts.

## Архитектурные решения

Главный документ проекта — [ADR-001: Архитектура AI-BI платформы n0](./ADR-001-n0.md). В нём описаны:

- выбор NATS вместо Kafka + service mesh;
- async execution model;
- tenant isolation и zero-trust подход;
- Redis/S3 result strategy;
- plugin architecture;
- отказоустойчивость и горизонтальное масштабирование;
- целевой security и audit контур.

Если реализация и ADR расходятся, таблица статуса в этом README показывает текущее состояние кода, а ADR — целевое направление.

## Название

`n0` читается как **«эн-ноль»**. Название происходит от нулевой индексации массивов: элемент `0` является первым элементом коллекции. Так и n0 задуман как первый, основополагающий элемент в области, которую представляет проект, — безопасной работы AI-агентов с данными.

## Участие в разработке

Перед изменением публичного API:

1. обновите protobuf-контракт или REST handler;
2. добавьте tenant/security regression test;
3. выполните `make proto`, если менялись `.proto`;
4. запустите `make test-race`;
5. проверьте Web Admin через `npm run lint && npm run build`;
6. обновите README и ADR, если меняется архитектурное обещание.

Новые database adapters должны реализовывать единый adapter contract и не передавать credentials через публичный API.

---

**n0 — безопасный execution layer для агентов, которые работают с реальными данными.**
