---
title: "Как дать AI-агенту доступ к данным, не отдавая ему пароль от production"
published: false
description: "Разбираем архитектуру n0 — Go-платформы, которая превращает доступ AI-агента к корпоративным данным в контролируемый MCP-инструмент с tenant isolation и асинхронными query jobs."
tags: ai, go, security, databases, mcp
---

AI-агент умеет написать SQL за несколько секунд.

Но это ещё не значит, что ему нужно дать:

- connection string от production;
- постоянный пароль к базе;
- возможность выполнить любой SQL;
- доступ к данным другого пользователя;
- ответственность за retries, timeouts и хранение результатов.

На практике между агентом и базой нужен не «умный прокси», а отдельная trust boundary.

В этой заметке я покажу, как устроен `n0` — open-source execution layer на Go для безопасного доступа AI-агентов к корпоративным данным. Недавно в нём появился MCP server, поэтому агент может работать с данными через стандартные tools, но сама модель доступа, проверки и выполнения остаётся внутри платформы.

## Коротко: что строим

Идея выглядит так:

```text
AI agent
   │
   │ MCP / Streamable HTTP
   ▼
Agent Gateway
   │  JWT, tenant context, tool routing
   ├──────────────► Meta Service
   │                 catalog, workspaces, schema
   │
   └──────────────► Query Engine
                     sandbox, jobs, results
                           │
                           ▼
                    Connection Manager
                           │
                           ▼
                 PostgreSQL / MySQL / ClickHouse / ...
```

Агент решает, **что спросить**. `n0` решает, **можно ли это сделать, в чьём контексте и как именно выполнить запрос**.

## Почему прямое подключение агента к базе — плохая граница

Самый быстрый прототип обычно выглядит так: сгенерировать SQL, передать его в SDK базы и вернуть результат модели.

Проблемы начинаются сразу после первого демо.

Во-первых, секрет оказывается слишком близко к агенту. Даже если модель сама его не «видит», connection string начинает жить в конфигурации интеграции, логах, переменных окружения или инструментах отладки.

Во-вторых, SQL — это не только `SELECT`. Ошибка в prompt или валидации может превратить аналитический запрос в `DROP`, `UPDATE`, блокирующую транзакцию или чтение слишком большого объёма данных.

В-третьих, синхронный HTTP-запрос плохо подходит для аналитики. Запрос может выполняться дольше таймаута клиента, а результат нужно где-то хранить, повторно отдавать страницами и связывать с аудитом.

И наконец, многопользовательская система должна проверять владельца не только у connection, но и у job, schema snapshot и результата.

Поэтому в `n0` MCP — это только внешний протокол. Он не становится новой дырой в безопасности: tool call попадает в тот же Agent Gateway, что и REST API.

## MCP endpoint в Agent Gateway

Сервер доступен на том же публичном порту:

```text
http://localhost:8083/mcp
```

Транспорт — Streamable HTTP. Endpoint проходит через существующий JWT middleware, а в production запросы без `Authorization: Bearer <token>` отклоняются.

Сейчас агенту доступны шесть инструментов:

| Tool | Назначение |
| --- | --- |
| `get_schema` | получить доступную схему connection |
| `submit_query` | отправить read-only SQL как асинхронную job |
| `get_query_status` | проверить состояние job |
| `get_query_result` | получить страницу результата |
| `list_connections` | перечислить connections без credentials |
| `list_workspaces` | перечислить доступные workspaces |

Обратите внимание: в списке нет инструмента «выполнить произвольный SQL напрямую». Даже `submit_query` только создаёт job, а сам SQL потом проходит через Query Engine sandbox.

## Как выглядит вызов tool

Для ручной проверки можно отправить JSON-RPC запрос напрямую. Здесь `$TOKEN` — JWT пользователя или agent token.

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

Ответ содержит `job_id`, а не большой массив строк:

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

Дальше агент вызывает `get_query_status`, ждёт terminal status и забирает результат через `get_query_result` с пагинацией. Такой контракт лучше соответствует аналитическим нагрузкам, чем попытка удерживать один HTTP-запрос открытым до завершения работы базы.

## Что происходит внутри tool call

Важная часть архитектуры — MCP handler не ходит в базу самостоятельно. Он только переводит внешний вызов в уже существующие внутренние клиенты:

```go
mcp.AddTool(server, &mcp.Tool{
    Name:        "submit_query",
    Description: "Submit a tenant-scoped read-only SQL query",
}, s.mcpSubmitQuery)
```

А обработчик формирует внутренний protobuf-запрос:

```go
resp, err := s.queryCli.SubmitQuery(ctx, &pb.SubmitQueryRequest{
    TenantId:     mcpTenantID(ctx),
    ConnectionId: input.ConnectionID,
    Sql:          input.SQL,
})
```

Здесь принципиально важен `TenantId`. Он берётся из проверенного JWT-контекста, а не из аргументов MCP tool. Агент не может передать чужой `tenant_id` и тем самым изменить область авторизации.

## Где реально применяются ограничения

После gateway запрос попадает в Query Engine. Текущий sandbox:

- принимает только один `SELECT`;
- блокирует DDL и DML: `CREATE`, `ALTER`, `DROP`, `INSERT`, `UPDATE`, `DELETE`, `TRUNCATE`;
- отклоняет несколько SQL statements;
- ограничивает `LIMIT` и добавляет его, если запрос его не содержит;
- проверяет таблицы по `query_policy.allowed_tables`;
- может добавить tenant predicate по настроенной колонке;
- передаёт timeout до драйвера базы.

Это defence in depth. Даже если агент сгенерировал опасный SQL, одного JWT и наличия connection недостаточно, чтобы запрос был выполнен.

При этом credentials остаются внутри Connection Manager. В публичных ответах параметры connection редактируются, поэтому вызовы `list_connections` и REST API не возвращают пароли.

## Почему здесь NATS и асинхронные jobs

`n0` разделяет orchestration и execution.

Agent Gateway быстро принимает запрос и передаёт его Query Engine. Query Engine создаёт job в NATS JetStream, worker забирает её и выполняет через Connection Manager. Metadata, состояние job и результаты живут отдельно от процесса gateway.

Это даёт несколько полезных свойств:

1. Gateway не держит долгий запрос открытым.
2. Worker можно масштабировать независимо от входящего RPS.
3. Результат можно получать страницами.
4. Retry и audit становятся частью жизненного цикла job.
5. MCP и REST используют одну и ту же бизнес-логику, а не две разные реализации безопасности.

## Что уже работает, а что пока нет

В текущем состоянии доступны:

- REST API и Web Admin;
- JWT authentication для users и agents;
- schema discovery;
- tenant-scoped connections и query jobs;
- sandbox для аналитического SQL;
- адаптеры PostgreSQL, MySQL, ClickHouse, SQLite, MSSQL и BigQuery;
- Streamable HTTP MCP server в Agent Gateway;
- пагинация результатов, audit pipeline и plugin registry foundation.

Проект всё ещё не пытается изображать полностью готовый managed cloud. В roadmap остаются Vault leases, PostgreSQL RLS для metadata, production Kubernetes/HA topology, distributed rate limiting и полноценный lifecycle для agent tokens.

Это важное различие: working MCP endpoint уже есть, но production security всё равно требует read-only ролей в source databases, TLS, network policies, секретов вне Compose и нормальной операционной модели.

## Запуск локально

```bash
git clone https://github.com/sickagent/n0.git
cd n0
cp .env.example .env
make up
make migrate-up
```

После запуска:

- Web Admin: `http://localhost:3000`;
- REST API: `http://localhost:8083`;
- MCP: `http://localhost:8083/mcp`.

Подробный walkthrough с регистрацией пользователя, созданием workspace и connection находится в [README](https://github.com/sickagent/n0#quick-start), а архитектурные решения — в [ADR-001](https://github.com/sickagent/n0/blob/main/ADR-001-n0.md).

## Вместо вывода

Хорошая интеграция AI с данными — это не «подключить LLM к PostgreSQL».

Это задача о границах доверия:

- агент должен видеть только те capabilities, которые ему разрешены;
- credentials не должны пересекать execution boundary;
- запрос должен пройти policy checks до базы;
- длинная работа должна быть job, а не случайным зависшим HTTP request;
- каждая операция должна иметь tenant context и audit trail.

MCP хорошо решает задачу интерфейса между агентом и системой. Но безопасность появляется не из MCP самого по себе, а из архитектуры вокруг него.

Именно поэтому в `n0` MCP tool — это тонкий вход в Agent Gateway, а не прямой путь к данным.

Исходный код: [github.com/sickagent/n0](https://github.com/sickagent/n0)

Если строите AI data analyst или свой agent platform, буду рад обсудить, где у вас проходит граница между агентом, policy layer и базой данных.
