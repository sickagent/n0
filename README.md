# n0

[Русский](./README.ru.md) · **English**

[![CI](https://github.com/sickagent/n0/actions/workflows/ci.yml/badge.svg)](https://github.com/sickagent/n0/actions/workflows/ci.yml)
[![Container images](https://github.com/sickagent/n0/actions/workflows/docker.yml/badge.svg)](https://github.com/sickagent/n0/actions/workflows/docker.yml)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev/)

**Data becomes an agent tool—without giving agents direct access to your databases.**

n0 is a Go-based AI-BI platform for connecting AI agents to enterprise data safely. An agent discovers the schemas it is allowed to use, submits analytical SQL as an asynchronous job, and receives structured results ready for analysis, charts, and reports.

n0 is not another chatbot or dashboard builder. It is an execution and metadata layer between agents and data sources. The platform owns authentication, tenant isolation, connection management, query validation, execution, job lifecycle, and result delivery.

> n0 is under active development. The primary end-to-end flow, policy-enforced query execution, durable result storage, plugin health routing, and audit persistence work today. MCP, Vault, metadata-table RLS, and Kubernetes deployment remain on the roadmap.

[Features](#features) · [Quick start](#quick-start) · [First API query](#your-first-query-through-the-api) · [Architecture](#how-a-query-runs) · [Security](#production-security) · [Development](#development) · [Roadmap](#roadmap)

## What is n0?

A typical AI agent can write SQL, but it should not:

- know long-lived production database passwords;
- connect directly to data sources;
- execute arbitrary DDL or DML;
- see another tenant's data or jobs;
- own retry, timeout, persistence, and audit behavior;
- keep critical coordination state inside a single process.

n0 places a controlled execution layer between the agent and the data:

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

The agent decides **what to ask**. The platform controls **who may access the data, whether the query is allowed, and how it is executed**.

## Why n0?

Most AI-to-data integrations begin by giving an agent a connection string and end as a collection of scripts without a shared security model. That approach breaks down quickly when a system gains multiple users, tenants, data sources, and deployment environments.

n0 is built around four principles:

1. **Zero trust toward agents.** An agent never receives direct database access and is never treated as a trusted party.
2. **Asynchronous execution.** An analytical query is a job with its own status and result—not a long-running HTTP request.
3. **Tenant isolation.** Connection and job ownership is checked on every public operation.
4. **An extensible platform.** New sources and capabilities should be added through adapters and plugins instead of changes to core services.

The full architectural rationale is documented in [ADR-001-n0.md](./ADR-001-n0.md).

## Use cases

- **AI data analyst.** A corporate assistant or agent discovers a schema and answers questions without receiving a raw connection string.
- **Embedded analytics.** A product submits analytical jobs through a stable API and renders structured results in its own interface.
- **Self-service data access.** Teams register approved sources and run constrained read-only queries through one gateway.
- **Agent platform.** An internal platform connects several agent types to a shared catalog, job queue, and access policy.
- **Adapter ecosystem.** A team adds a proprietary warehouse or internal data source without exposing implementation details to every client.

## What n0 is not

- It is not an LLM and does not generate SQL—the model and agent remain external.
- It is not a complete BI frontend—clients may build charts and dashboards on top of returned data.
- It is not a general-purpose database proxy—the public path is designed for controlled analytical workloads.
- It is not yet a managed cloud service—the owner currently deploys and operates the production infrastructure.

## Features

- **Connection catalog** — create, inspect, list, and delete tenant-scoped data connections.
- **Schema discovery** — retrieve tables and columns through one API and persist schema snapshots.
- **Asynchronous SQL jobs** — submit, poll status, and retrieve paginated results.
- **Query guardrails** — single `SELECT` statements, finite row limits, and dangerous-operation blocking.
- **Built-in adapters** — PostgreSQL, MySQL, ClickHouse, SQLite, Microsoft SQL Server, and BigQuery.
- **Web Admin** — manage accounts, workspaces, connections, plugins, and queries.
- **JWT authentication** — separate user and agent claims with issuer, audience, expiration, and algorithm validation.
- **Encrypted connection configuration** — AES-256-GCM protection for stored connection parameters.
- **NATS JetStream workers** — durable consumers, explicit acknowledgements, and bounded redelivery.
- **Plugin registry foundation** — register external adapters and agent capabilities.
- **Production lifecycle defaults** — bounded HTTP timeouts, request-size limits, graceful shutdown, and NATS reconnect/drain.
- **Observability baseline** — structured logs, Prometheus metrics, and health endpoints.

## Current status

This README deliberately separates implemented behavior from the target architecture.

| Area | Status | Available today |
|---|---:|---|
| REST API and Web Admin | ✅ Working | Authentication, connections, schemas, plugins, Query Lab |
| Asynchronous query jobs | ✅ Working | JetStream submit, workers, polling, pagination |
| Tenant isolation | ✅ Baseline | Connections and jobs are checked against `tenant_id` |
| Built-in database adapters | ✅ Working | PostgreSQL, MySQL, ClickHouse, SQLite, MSSQL, BigQuery |
| Credential protection | ✅ Baseline | AES-256-GCM at rest and redaction at the public boundary |
| Query sandbox | ✅ Implemented | Structural SELECT parser, table allowlists, tenant predicate injection, statement/limit enforcement |
| Plugin platform | ✅ Implemented | Registration validation, persistent lifecycle, gRPC health probes, automatic route add/remove |
| Result persistence | ✅ Implemented | Redis job metadata and small results; S3-compatible storage for large payloads, both with retention |
| Audit pipeline | ✅ Implemented | JetStream producer acknowledgement, durable consumer, explicit ack, idempotent PostgreSQL sink |
| MCP server | ⏳ Roadmap | Included in the architecture but not connected to Agent Gateway yet |
| Vault integration | ⏳ Roadmap | Configuration exists; runtime lease management is not implemented yet |
| Kubernetes / HA | ⏳ Roadmap | Manifests, HPA, PDB, mTLS, and a production NATS topology are still needed |

## Quick start

### Requirements

- Docker Engine with Docker Compose v2;
- Go 1.26+ if you run services outside Docker;
- Node.js 22+ for local Web Admin development;
- `make` and `curl`; `jq` is useful for the examples below.

### 1. Prepare the configuration

```bash
cp .env.example .env
```

The values in `.env.example` are intended only for local development. Never reuse its development JWT/AES keys or database passwords in production.

### 2. Start the infrastructure and services

```bash
make up
make migrate-up
```

Check the containers:

```bash
docker compose -f deployments/docker-compose.yml ps
```

### 3. Open the application

- Web Admin: [http://localhost:3000](http://localhost:3000)
- Agent Gateway REST API: [http://localhost:8083](http://localhost:8083)
- NATS monitoring: [http://localhost:8222](http://localhost:8222)

Create a user with a password between 12 and 64 characters. The platform automatically creates a `Default Workspace` after registration.

Stop the local environment with:

```bash
make down
```

## Your first query through the API

The following walkthrough covers the complete path from registration to a query result. Every public call goes through Agent Gateway.

### 1. Register and sign in

```bash
curl -sS -X POST http://localhost:8083/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"analyst@example.com","password":"change-me-123"}'

TOKEN=$(curl -sS -X POST http://localhost:8083/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"analyst@example.com","password":"change-me-123"}' \
  | jq -r '.token')
```

Public registration always creates a least-privileged `user`. Elevated roles must be assigned through a separate administrative workflow.

### 2. Get the default workspace

```bash
WORKSPACE_ID=$(curl -sS http://localhost:8083/v1/workspaces \
  -H "Authorization: Bearer $TOKEN" \
  | jq -r '.workspaces[0].id')
```

### 3. Create a connection

This example connects to the PostgreSQL container in the local Docker Compose environment:

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

Connection parameters are encrypted before they are stored and are not returned to clients in later connection reads.

### 4. Discover the schema

```bash
curl -sS "http://localhost:8083/v1/schema?connection_id=$CONNECTION_ID" \
  -H "Authorization: Bearer $TOKEN" | jq
```

### 5. Submit an asynchronous query

SQL is sent in a JSON body instead of a query string so that query text does not leak into URLs, browser history, and routine access logs.

```bash
JOB_ID=$(curl -sS -X POST http://localhost:8083/v1/query \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"connection_id\":\"$CONNECTION_ID\",\"sql\":\"SELECT now() AS server_time\"}" \
  | jq -r '.job_id')
```

### 6. Poll status and retrieve the result

```bash
curl -sS "http://localhost:8083/v1/query/status?job_id=$JOB_ID" \
  -H "Authorization: Bearer $TOKEN" | jq

curl -sS "http://localhost:8083/v1/query/result?job_id=$JOB_ID&page=1&page_size=100" \
  -H "Authorization: Bearer $TOKEN" | jq
```

Gateway derives the tenant context from the JWT. A user-supplied `tenant_id` is never treated as an authorization source.

## How a query runs

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
    C->>D: Read-only analytical query
    D-->>C: Rows
    C-->>Q: Structured result
    Q->>Q: Persist job state
    A->>G: GET status / result
    G->>Q: Tenant-scoped lookup
    Q-->>A: Status or result page
```

A worker acknowledges the message only after processing it. Poison payloads receive a terminal acknowledgement, while transient failures have a bounded number of redelivery attempts.

## Components

| Component | Responsibility | Technology |
|---|---|---|
| **Agent Gateway** | Public trust boundary: JWT, CORS, tenant context, REST orchestration | Go, Chi |
| **Meta Service** | Users, agents, workspaces, connections, schema snapshots, plugin registry | Go, PostgreSQL |
| **Query Engine** | Job lifecycle, SQL guardrails, workers, result pagination | Go, NATS JetStream |
| **Connection Manager** | Adapter selection, connection testing, schema discovery, execution | Go, database drivers |
| **Web Admin** | Platform management and Query Lab | React, TypeScript, Vite, Mantine |
| **NATS** | Messaging, JetStream queues, discovery, coordination foundation | NATS Core + JetStream |
| **PostgreSQL** | Platform metadata | PostgreSQL 16 in development Compose |

All persisted platform tables use `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`. Join-table natural keys are retained as separate `UNIQUE` constraints, keeping identifiers uniform without losing domain-level deduplication.

### Why multiple services?

- Gateway can scale on incoming request volume independently of query workers.
- Connection Manager isolates database drivers and credentials from the public API.
- Query Engine can scale against queue depth.
- Meta Service remains the single owner of catalog and tenancy metadata.

Internal services must never be exposed through a public ingress. Production access should be restricted with network policy and service-to-service identity.

## Data adapters

| Adapter type | State | Primary parameters |
|---|---:|---|
| `postgres` | Built in | host, port, user, password, database, sslmode |
| `mysql` | Built in | host, port, user, password, database |
| `clickhouse` | Built in | host, port, user, password, database |
| `sqlite` | Built in | path |
| `mssql` | Built in | host, port, user, password, database |
| `bigquery` | Built in | project_id, location, and provider credentials |

The adapter registry lives in [services/connection-manager/internal/registry](./services/connection-manager/internal/registry). External plugin contracts are defined in [proto/n0/platform/v1/plugin.proto](./proto/n0/platform/v1/plugin.proto).

External database adapters use a persisted lifecycle: registration validates the contract and endpoint, Meta Service probes the standard gRPC Health service every 15 seconds, and three consecutive failures move the plugin to `degraded`. `active` routes are broadcast to Connection Manager; degraded or disabled routes are removed immediately. A restarted Connection Manager is repopulated by the recurring health cycle. External plugins must implement the standard `grpc.health.v1.Health` service in addition to the n0 plugin contract.

A runnable Go implementation is available in [example/plugin](./example/plugin).

## Result and audit durability

Query jobs, states, and small result sets are stored in Redis with the configured TTL. Results larger than `RESULT_INLINE_MAX_BYTES` are written to the `results/` prefix of the configured S3-compatible bucket; startup installs a matching bucket expiration policy. Production startup fails if Redis is unavailable, and oversized results fail closed if object storage is not configured.

Every terminal query execution publishes a tenant-partitioned event to `audit.events.{tenant}` using JetStream's acknowledged publish API. Meta Service consumes through the durable `postgres-audit-sink` consumer, inserts the event into `audit_events` with its stable event ID, and acknowledges the message only after the transaction succeeds. Redelivery is idempotent through the table's primary key.

## Query guardrails

The current sandbox implementation:

- accepts only a query beginning with `SELECT`;
- rejects multiple SQL statements;
- blocks DDL and DML such as `CREATE`, `ALTER`, `DROP`, `INSERT`, `UPDATE`, `DELETE`, and `TRUNCATE`;
- blocks `COPY`, `CALL`, `EXECUTE`, `MERGE`, `SELECT INTO`, and locking reads;
- injects `LIMIT 10000` when no limit is present;
- rejects non-numeric or excessive limits;
- propagates an execution timeout to the database driver.

The Query Engine parses a conservative analytical SELECT subset, extracts every base table, checks it against the connection's `query_policy.allowed_tables`, rejects unsupported constructs, and caps `LIMIT`. When `query_policy.tenant_column` is configured, it injects a tenant predicate while preserving boolean precedence. Unsupported or ambiguous SQL is rejected. Keep database credentials read-only as an independent defence-in-depth boundary.

Example connection policy:

```json
{
  "query_policy": {
    "allowed_tables": ["public.orders", "public.customers"],
    "tenant_column": "tenant_id"
  }
}
```

## Configuration

Configuration is supplied through flags or environment variables. Both regular names and namespaced `N0_*` names are supported; namespaced values take precedence.

| Variable | Purpose | Development default |
|---|---|---|
| `ENVIRONMENT` | `development` or `production` | `development` |
| `LOG_LEVEL` | debug, info, warn, error | `info` |
| `NATS_URL` | NATS endpoint | `nats://localhost:4222` |
| `POSTGRES_DSN` | Metadata PostgreSQL DSN | local PostgreSQL |
| `JWT_SECRET` | Base64 HMAC key, at least 32 decoded bytes | development key in `.env.example` |
| `JWT_EXPIRY_HOURS` | Access token lifetime | `24` |
| `ENCRYPTION_KEY` | Base64 AES-256 key for connection config | development key in `.env.example` |
| `CORS_ALLOWED_ORIGINS` | Comma-separated browser origin allowlist | `http://localhost:3000` |
| `META_SERVICE_ADDR` | Meta Service gRPC endpoint | `localhost:8080` |
| `META_SERVICE_HTTP_URL` | Internal Meta HTTP base URL | `http://localhost:8081` |
| `QUERY_ENGINE_ADDR` | Query Engine gRPC endpoint | `localhost:8082` |
| `CONNECTION_MANAGER_ADDR` | Connection Manager gRPC endpoint | `localhost:8081` |
| `WORKER_COUNT` | Number of query workers | `4` |
| `REDIS_MODE` | Redis topology: `standalone` or `cluster` | `standalone` |
| `REDIS_ADDR` | Standalone Redis address (also a backwards-compatible cluster seed) | `localhost:6379` |
| `REDIS_ADDRS` | Comma-separated Redis Cluster seed addresses; overrides `REDIS_ADDR` | empty |
| `REDIS_USERNAME` | Optional Redis ACL username | empty |
| `REDIS_PASSWORD` | Optional Redis password | empty |
| `REDIS_DB` | Redis database; must be `0` in cluster mode | `0` |
| `JOB_TTL_HOURS` | Job/result retention | `24` |
| `S3_ENDPOINT` | S3-compatible endpoint for large results | empty (disabled) |
| `S3_BUCKET` | Large-result bucket | `n0-results` |
| `RESULT_INLINE_MAX_BYTES` | Redis-to-object-storage threshold | `1048576` |
| `VAULT_ADDR` | Reserved for Vault integration | `http://localhost:8200` |

For Redis Cluster, set for example `REDIS_MODE=cluster` and
`REDIS_ADDRS=redis-0:6379,redis-1:6379,redis-2:6379`. The addresses are seed
nodes; the client discovers the remaining cluster topology automatically.

Invalid integer and boolean environment values cause a startup error instead of being silently ignored.

## Production security

When `ENVIRONMENT=production`:

- Agent Gateway refuses to start without a valid `JWT_SECRET`;
- Meta Service refuses to start without an `ENCRYPTION_KEY`;
- the decoded JWT secret must contain at least 32 bytes;
- the encryption key must contain exactly 32 bytes for AES-256-GCM.

Before a production deployment, you must also:

- keep keys in Kubernetes Secrets, Vault, or a cloud secret manager;
- use different JWT and encryption keys;
- enable TLS for ingress, NATS, and PostgreSQL;
- isolate internal APIs with network policies;
- configure NATS Accounts/ACLs and tenant-specific subjects;
- create read-only roles in every source database;
- enable PostgreSQL RLS for metadata tables;
- size JetStream, PostgreSQL audit retention, Redis, and object storage for the expected workload;
- define backup, restore, and key-rotation procedures;
- add distributed rate limiting and external quota storage;
- remove or replace every development credential from Compose.

The Docker Compose configuration in this repository is an integration environment for development—not a production topology.

## API overview

### Public authentication

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/v1/auth/register` | Create a user |
| `POST` | `/v1/auth/login` | Obtain a JWT |
| `GET` | `/v1/auth/me` | Read the current user |

### Connections and catalog

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/v1/workspaces` | List accessible workspaces |
| `GET` | `/v1/connections` | List connections |
| `POST` | `/v1/connections` | Create a connection |
| `GET` | `/v1/connections/{id}` | Read a connection without credentials |
| `DELETE` | `/v1/connections/{id}` | Delete a connection owned by the tenant |
| `POST` | `/v1/test-connection` | Validate parameters before saving |
| `GET` | `/v1/schema?connection_id=...` | Retrieve a schema snapshot |

### Query jobs

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/v1/query` | Create an asynchronous job |
| `GET` | `/v1/query/status?job_id=...` | Read tenant-scoped status |
| `GET` | `/v1/query/result?job_id=...` | Read a tenant-scoped result page |

### Agents and plugins

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/v1/agents` | List the user's agents |
| `POST` | `/v1/agents` | Register an agent |
| `POST` | `/v1/agents/{id}/token` | Issue an agent token |
| `POST` | `/v1/plugins/register` | Register a plugin definition |

In a production configuration, every endpoint except registration, login, and health checks requires `Authorization: Bearer <token>`.

## Development

### Repository layout

```text
n0/
├── ADR-001-n0.md
├── README.md               # English documentation
├── README.ru.md            # Russian documentation
├── deployments/
│   └── docker-compose.yml
├── pkg/shared/
│   ├── config/             # flags + environment loading
│   ├── crypto/             # AES-256-GCM
│   ├── discovery/          # NATS service discovery
│   ├── graceful/           # shutdown contexts
│   ├── httpserver/         # hardened HTTP defaults
│   ├── jwt/                # user and agent tokens
│   ├── natsclient/         # Core, JetStream, and KV client
│   └── observability/      # metrics and health
├── proto/
│   ├── n0/platform/v1/     # source protobuf contracts
│   └── gen/go/             # committed generated Go module
├── services/
│   ├── agent-gateway/
│   ├── connection-manager/
│   ├── meta-service/
│   ├── query-engine/
│   └── web-admin/
└── tests/e2e/
```

### Main commands

| Command | Action |
|---|---|
| `make up` | Build and start the local stack |
| `make down` | Stop the local stack |
| `make migrate-up` | Apply metadata migrations |
| `make proto` | Generate Go protobuf contracts |
| `make build` | Build all Go services into `bin/` |
| `make docker-build` | Build service images |
| `make test` | Run tests for every Go module |
| `make test-race` | Run the Go race detector |
| `make lint` | Run golangci-lint for every Go module |
| `make tidy` | Synchronize Go workspace dependencies |

### Run one service

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

Validate the production bundle with:

```bash
npm run lint
npm run build
```

## Testing and CI

CI performs:

- deterministic protobuf generation and drift detection;
- unit tests for every Go module;
- the Go race detector;
- `go vet`;
- ESLint for Web Admin;
- a TypeScript and Vite production build.

The container workflow builds every service for `linux/amd64` and
`linux/arm64`. Pull requests are build-only; pushes to `main` and version tags
publish images such as `ghcr.io/sickagent/n0-agent-gateway` to GitHub Container
Registry. Dependabot monitors Go, npm, Docker, and GitHub Actions dependencies.

Run the main checks locally:

```bash
make test
make test-race

cd services/web-admin
npm run lint
npm run build
```

The end-to-end suite lives in [tests/e2e](./tests/e2e) and requires the Docker Compose stack to be running.

## Roadmap

### Security and tenancy

- PostgreSQL Row-Level Security for metadata;
- additional SQL dialect coverage beyond the conservative portable SELECT subset;
- mTLS and service identity between internal services;
- agent ownership verification and token revocation lifecycle;
- distributed rate limiting and quotas through NATS KV or Redis.

### Reliability and scale

- optional result compression, storage-class transitions, replication, and signed download URLs;
- idempotency keys and deduplication by `job_id`;
- the retry and dead-letter topology defined in the ADR;
- Kubernetes deployments, HPA, PDB, and readiness probes;
- a replicated production NATS cluster with Accounts.

### Agent platform

- an MCP server in Agent Gateway;
- an external gRPC API for enterprise agents;
- dynamic capability discovery;
- complete plugin lifecycle: validation, health, degraded, deprecated, revoked;
- query transformer hooks and an isolated WASM runtime.

### Governance and observability

- audit archival/export integrations and configurable retention policies;
- OpenTelemetry traces across gateway, queue, and worker;
- per-tenant usage metrics and budgets;
- credential rotation through Vault leases;
- operational dashboards and SLO alerts.

## Architecture decisions

The main design document is [ADR-001: n0 AI-BI platform architecture](./ADR-001-n0.md). It covers:

- choosing NATS over Kafka plus a service mesh;
- the asynchronous execution model;
- tenant isolation and zero-trust design;
- the Redis/S3 result strategy;
- plugin architecture;
- fault tolerance and horizontal scaling;
- the target security and audit model.

When implementation and ADR differ, the status table in this README describes the current code, while the ADR describes the intended destination.

## The name

`n0` is pronounced **“en-zero.”** The name comes from zero-based array indexing: element `0` is the first element in a collection. In the same way, n0 is intended to be the first foundational element in the domain this project represents—AI agents working safely with data.

## Contributing

Before changing the public API:

1. update the protobuf contract or REST handler;
2. add a tenant/security regression test;
3. run `make proto` if a `.proto` file changed;
4. run `make test-race`;
5. validate Web Admin with `npm run lint && npm run build`;
6. update the README and ADR when an architectural promise changes.

New database adapters should implement the shared adapter contract and must never pass credentials through the public API.

---

**n0 is a secure execution layer for agents working with real data.**
