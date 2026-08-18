---
title: "I Didn't Want My AI Agent to Have a Database Password, So I Built a Gateway"
published: false
description: "A builder's note on putting an MCP server, tenant isolation, SQL guardrails, and asynchronous jobs between an AI agent and enterprise databases."
tags: ai, go, security, mcp
---

The first version of the idea was embarrassingly simple.

Give the agent a database URL, let it write SQL, run the query, and send the rows back to the model.

It would have made a nice demo. It also would have made a terrible system.

The problem wasn't only that a model might produce a `DROP TABLE`. A read-only user can still run a query that scans half a warehouse, hold connections open, join against a table it was never supposed to see, or return far more data than the agent actually needs.

And a database password is still a database password, even when it is hidden inside an agent configuration file.

That was the starting point for [n0](https://github.com/sickagent/n0): an open-source Go platform that sits between AI agents and enterprise data. The agent gets a small set of useful tools. The gateway owns authentication and tenant context. A separate query service decides whether the SQL is safe to execute.

This is a note about the architecture, but also about the decisions behind it. The project is working software, not a claim that every production concern has been solved.

## The mental model

I keep coming back to one sentence:

> The agent decides what it wants to ask. The platform decides whether it is allowed to ask it and how the request is executed.

The rough request path looks like this:

```text
AI agent
   │
   │ MCP / Streamable HTTP
   ▼
Agent Gateway
   │  JWT, tenant context, tool routing
   ├──────────────► Meta Service ─────► PostgreSQL
   │                 workspaces,        metadata
   │                 connections, schema
   │
   └──────────────► Query Engine ─────► NATS JetStream
                     SQL sandbox,       asynchronous jobs
                     result lifecycle
                           │
                           ▼
                    Connection Manager
                           │
                           ▼
                 PostgreSQL / MySQL / ClickHouse / ...
```

There are a few more boxes in the repository, but these are the boundaries that matter for an agent.

The MCP server lives in the Agent Gateway. It does not open a database connection, inspect credentials, or implement a second query executor. It translates MCP calls into the same internal clients used by the REST API.

That last part is intentional. I didn't want security rules to slowly diverge between "the API path" and "the AI path".

## Why MCP is not the security boundary

MCP is a useful way to describe capabilities to an agent. It gives the client a standard way to discover tools and call them.

It does not answer the important questions for a data platform:

- Who is calling?
- Which tenant do they belong to?
- Which connection can they use?
- Which tables are allowed?
- Is this query safe and bounded?
- Where does the result live while the query is running?

Those questions belong to the gateway and the services behind it.

In n0, the MCP endpoint is exposed at:

```text
http://localhost:8083/mcp
```

It goes through the same JWT middleware as the REST API. The tenant ID used for internal requests comes from the verified JWT context, not from a `tenant_id` argument supplied by the agent.

That distinction is easy to miss. If a tool accepts both `connection_id` and `tenant_id`, the model can accidentally—or deliberately—ask for a different tenant's data. The public tool doesn't accept that field at all.

## The tools are deliberately boring

The current MCP server exposes six tools:

| Tool | What it does |
| --- | --- |
| `get_schema` | Returns the schema visible for a connection |
| `submit_query` | Creates an asynchronous read-only query job |
| `get_query_status` | Checks the state of a query job |
| `get_query_result` | Fetches a paginated result page |
| `list_connections` | Lists connections without returning credentials |
| `list_workspaces` | Lists workspaces in the current tenant |

There is no `execute_sql_now` tool. There is no tool that returns a connection string. There is no tool that lets the model choose a different tenant.

The boringness is a feature. Tools are part of the security model, not just conveniences for prompting.

## A real tool call

Once the gateway is running, a client can call `submit_query` with a normal MCP JSON-RPC request. `$TOKEN` can be a user JWT or an agent token issued by n0.

```bash
curl -sS -X POST http://localhost:8083/mcp \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "submit_query",
      "arguments": {
        "connection_id": "conn_123",
        "sql": "SELECT customer_id, sum(amount) AS revenue FROM public.orders GROUP BY customer_id"
      }
    }
  }'
```

The response is intentionally small:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "structuredContent": {
      "job_id": "job_456",
      "status": "pending"
    }
  }
}
```

The agent polls `get_query_status`, then asks for pages with `get_query_result`. This is a better fit for analytical work than holding one HTTP request open while a warehouse does its thing.

## The implementation is a thin adapter

The MCP registration is intentionally small. The official Go SDK handles the protocol details and typed tool schemas; the handler calls the existing Query Engine client.

```go
mcp.AddTool(server, &mcp.Tool{
    Name:        "submit_query",
    Description: "Submit a tenant-scoped read-only SQL query",
}, s.mcpSubmitQuery)
```

The interesting part is not the registration. It is the context propagation:

```go
resp, err := s.queryCli.SubmitQuery(ctx, &pb.SubmitQueryRequest{
    TenantId:     mcpTenantID(ctx),
    ConnectionId: input.ConnectionID,
    Sql:          input.SQL,
})
```

The MCP request context has already passed through JWT verification. The handler doesn't trust a tenant field from the tool arguments and doesn't try to reimplement authorization in the MCP package.

This also means the same internal client can be exercised from REST, MCP, and tests. Fewer paths are good. Fewer security models are better.

## What happens to the SQL?

`submit_query` only creates a job. Query Engine is the component that validates and executes the statement.

The current sandbox is intentionally conservative. It:

- accepts one `SELECT` statement;
- rejects DDL and DML such as `CREATE`, `DROP`, `INSERT`, `UPDATE`, and `DELETE`;
- rejects multiple statements and locking reads;
- adds a finite `LIMIT` when one is missing;
- rejects excessive or malformed limits;
- checks referenced tables against the connection policy;
- can inject a tenant predicate when a policy defines a tenant column;
- propagates an execution timeout to the database driver.

The source database should still use a read-only role. Application checks are not a replacement for database permissions. They are another layer.

This is the part where I resist the temptation to say "the query is safe". The honest version is: the query passed the checks we currently enforce, and the database role and network boundaries are still important.

## Why make queries asynchronous?

For a tiny demo, synchronous execution is simpler. For a platform, it creates a pile of awkward edge cases.

What happens when the client disconnects while the database is still working? What happens when the result is larger than the response body? What happens when the worker restarts after the query has been accepted? How do you retry without running the same expensive query twice?

Turning the request into a job gives the system somewhere to put those answers.

The flow is roughly:

1. Agent Gateway authenticates the caller and submits the request.
2. Query Engine publishes a durable job through NATS JetStream.
3. A worker validates and executes the SQL through Connection Manager.
4. Job state and results are persisted separately from the gateway process.
5. The agent polls status and reads paginated results.
6. Terminal events are sent through the audit pipeline.

It is more code than `db.QueryContext`. It is also much easier to reason about once there is more than one user and more than one query running.

## The awkward parts I am not hiding

The current version has working MCP support, but it is not a finished cloud product.

There are still open areas:

- Vault-backed runtime credential leases;
- PostgreSQL RLS for metadata tables;
- distributed rate limits and quotas;
- production Kubernetes and high-availability manifests;
- agent token ownership verification and revocation lifecycle;
- dynamic capability discovery for plugins.

The MCP transport is stateless so gateway replicas do not depend on an in-memory session store. That keeps the first deployment model simple, but it also means the rest of the production story—rate limiting, revocation, observability, and operational policy—still needs to be designed at platform level.

I prefer saying this out loud. A working endpoint is not the same thing as a completed security program.

## Running it locally

```bash
git clone https://github.com/sickagent/n0.git
cd n0
cp .env.example .env
make up
make migrate-up
```

The local stack exposes:

- Web Admin at `http://localhost:3000`;
- REST API at `http://localhost:8083`;
- MCP at `http://localhost:8083/mcp`.

The repository README walks through registration, workspace creation, adding a connection, discovering its schema, and submitting a query. The architectural trade-offs are documented in [ADR-001](https://github.com/sickagent/n0/blob/main/ADR-001-n0.md).

## A small takeaway

MCP makes it easier for an agent to discover and call tools. It does not make direct database access safe.

The useful pattern, at least for this project, is:

```text
standard agent protocol
        +
strong identity and tenant context
        +
default-deny query policy
        +
asynchronous execution
        +
database-level least privilege
```

None of those layers is perfect on its own. That is exactly why they should not be collapsed into one clever prompt, one regex, or one database role.

If you are building an AI data analyst, the interesting question is probably not "which model writes the best SQL?"

It is "what is the smallest, most boring capability I can safely give the model?"

That is the question n0 is trying to answer.

The code is on [GitHub](https://github.com/sickagent/n0). Feedback, issues, and arguments about the boundaries are welcome.
