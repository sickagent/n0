# n0

Enterprise AI-BI platform. Agents connect via MCP or gRPC, discover data schemas, submit analytical queries, and receive structured results ready for visualization.

## Architecture

- **Agent Gateway** — MCP Server + gRPC/REST gateway, auth, rate limiting.
- **Meta Service** — Data catalog, plugin registry, schema snapshots (Go + PostgreSQL).
- **Query Engine** — Query planner, sandbox validator, worker pool, result formatter.
- **Connection Manager** — Connection pools, built-in and external database adapters, Vault integration.
- **NATS** — Internal messaging (JetStream queues, Core pub/sub, KV store, Request-Reply).
- **Redis** — Result cache and session state.
- **S3-compatible Object Storage** — Large result streaming.

## Quick Start

```bash
# Start infrastructure
make up

# Run migrations
make migrate-up

# Build all services
make build

# Or run individual services
cd services/meta-service && go run ./cmd
```

## Project Layout

```
n0/
├── deployments/docker-compose.yml
├── Makefile
├── go.work
├── .golangci.yml
├── proto/                      # Shared protobuf definitions
├── pkg/shared/                 # Shared libraries
│   ├── config/
│   ├── logger/
│   ├── natsclient/
│   ├── observability/
│   └── graceful/
└── services/
    ├── agent-gateway/
    ├── connection-manager/
    ├── meta-service/
    └── query-engine/
```
