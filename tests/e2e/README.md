# n0 Platform E2E Tests

Comprehensive end-to-end tests covering the full stack: agent-gateway, meta-service, connection-manager, query-engine and all database adapters.

## Prerequisites

All services must be running via Docker Compose:

```bash
cd /Users/n0byk/Desktop/lad/n0
make up
# or
docker compose -f deployments/docker-compose.yml up -d
```

Required healthy containers:
- `n0-agent-gateway` (:8083)
- `n0-meta-service` (:8080/:8085)
- `n0-connection-manager` (:8081/:8086)
- `n0-query-engine` (:8082/:8087)
- `n0-postgres` (:5432)
- `n0-mysql` (:3306)
- `n0-clickhouse` (:8123/:9000)
- `n0-nats` (:4222)
- `n0-redis` (:6379)

## Run Tests

```bash
cd /Users/n0byk/Desktop/lad/n0/tests/e2e
go test -v ./... -count=1
```

## Test Coverage

### Adapters (`TestAdapters_*`)
| Adapter | TestConnection | GetSchema | ExecuteQuery | Notes |
|---------|---------------|-----------|--------------|-------|
| **postgres** | ✅ | ✅ | ✅ | Real PostgreSQL 16 |
| **mysql** | ✅ | ✅ | ✅ | Real MySQL 8.0 |
| **clickhouse** | ✅ | ✅ | ✅ | Native protocol (:9000) + HTTP (:8123) for DDL setup |
| **sqlite** | ✅ | ✅ | ✅ | File-based temp DB |
| **mssql** | ✅ (expects failure) | — | — | No SQL Server in compose |
| **bigquery** | ✅ (expects failure) | — | — | Stub adapter |

### Admin API (`TestAdmin_*`)
- **Workspaces** – list workspaces, validate "Default Workspace" exists
- **Connections CRUD** – create → list → get → delete lifecycle via gateway
- **Plugins Register** – register plugin with capabilities

### Query Engine (`TestQuery_*`)
- **Submit Query** – create connection, submit SQL query via gateway, assert job ID returned

## Architecture

- `client.go` – thin HTTP wrapper with JSON helpers
- `adapters_test.go` – tabular tests for all 6 adapters
- `admin_test.go` – gateway admin endpoints
- `query_test.go` – query submission flow

Tests hit the public gateway (`:8083`) where possible, and connection-manager directly (`:8086`) for adapter smoke tests.
