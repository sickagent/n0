# Example database adapter plugin

This directory contains a minimal external n0 plugin written in Go. It implements
`n0.platform.v1.DatabaseAdapter` and the standard gRPC health service required by
the plugin lifecycle manager.

The adapter exposes one static table, `demo_metrics`, and accepts one query:

```sql
SELECT * FROM demo_metrics;
```

## Run locally

From the repository root:

```bash
go run ./example/plugin --addr :50051
```

Register it through the public gateway after logging in and exporting the token:

```bash
curl -X POST http://localhost:8083/v1/plugins/register \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "plugin_type": "DB_ADAPTER",
    "name": "example",
    "version": "0.1.0",
    "endpoint": "host.docker.internal:50051",
    "protocol": "grpc",
    "global": true
  }'
```

Use `localhost:50051` instead when the n0 services also run directly on the host.
After the health probe promotes the plugin to `active`, create a connection with
`adapter_type` set to `example` and params containing a non-empty `api_key`.

## Docker image

The build context must be the repository root because the plugin imports the
committed n0 protobuf module:

```bash
docker build -f example/plugin/Dockerfile -t n0/example-plugin:latest .
docker run --rm -p 50051:50051 n0/example-plugin:latest
```

## Verify

```bash
go test ./example/plugin/...
```
