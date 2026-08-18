# Kubernetes configuration

Сейчас сервисы n0 не читают YAML-файлы напрямую. Каждый Go-сервис запускается
через Cobra/Viper и получает настройки из flags или environment variables;
переменная `N0_<NAME>` имеет приоритет над `<NAME>`. Файлы
`services/*/config.example.yaml` — канонические примеры значений для Kubernetes,
а не самостоятельный runtime-формат.

## Как настройки попадают в Pod

Публичные и несекретные параметры кладутся в `ConfigMap`, секреты — в
`Secret`, после чего Deployment передаёт их контейнеру через `envFrom` или
точечные `env.valueFrom`. Например:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: n0-meta-service-config
data:
  ENVIRONMENT: "production"
  LOG_LEVEL: "info"
  NATS_URL: "nats://nats.n0.svc.cluster.local:4222"
  GRPC_ADDR: ":8080"
  GRPC_ADVERTISE_ADDR: "meta-service.n0.svc.cluster.local:8080"
  HTTP_ADDR: ":8081"
  CONNECTION_MANAGER_ADDR: "connection-manager.n0.svc.cluster.local:8080"
---
apiVersion: v1
kind: Secret
metadata:
  name: n0-meta-service-secrets
type: Opaque
stringData:
  POSTGRES_DSN: "postgres://n0_meta:<password>@postgres.n0.svc.cluster.local:5432/n0_meta?sslmode=require"
  ENCRYPTION_KEY: "<base64-encoded-32-byte-key>"
---
# Fragment of a Deployment
spec:
  template:
    spec:
      containers:
        - name: meta-service
          image: ghcr.io/sickagent/n0/meta-service:latest
          envFrom:
            - configMapRef:
                name: n0-meta-service-config
            - secretRef:
                name: n0-meta-service-secrets
          ports:
            - name: grpc
              containerPort: 8080
            - name: http
              containerPort: 8081
            - name: metrics
              containerPort: 9090
          readinessProbe:
            httpGet:
              path: /healthz
              port: http
          livenessProbe:
            httpGet:
              path: /healthz
              port: http
```

Не дублируйте `POSTGRES_DSN` в `ConfigMap` и `Secret`: в реальном Deployment
оставьте его только в `Secret`. Для `agent-gateway` секретом является
`JWT_SECRET`; для `meta-service` — `ENCRYPTION_KEY`; для `query-engine` —
Redis/S3 credentials. Секреты не следует коммитить в репозиторий.

## Service discovery внутри кластера

Для каждого Go-сервиса создаётся Kubernetes `Service` с DNS-именем вида
`<service>.<namespace>.svc.cluster.local`. Поэтому адреса из примеров работают
без IP-адресов Pod и переживают rolling update. `GRPC_ADVERTISE_ADDR` должен
указывать на DNS-имя Service, а не на Pod IP. Если адрес не задан, сервисы
используют локальный listen address как fallback, что подходит только для
одного процесса или локальной разработки.

Типовой набор Service-портов:

| Service | gRPC | HTTP | Metrics |
|---|---:|---:|---:|
| `agent-gateway` | 8080 | 8081 | 9090 |
| `meta-service` | 8080 | 8081 | 9090 |
| `query-engine` | 8080 | 8082 | 9090 |
| `connection-manager` | 8080 | 8082 | 9090 |

Наружу обычно публикуется только HTTP `agent-gateway` через Ingress. Остальные
HTTP/gRPC endpoints должны оставаться ClusterIP и быть ограничены NetworkPolicy.

## Как работает масштабирование

- `agent-gateway` масштабируется по HTTP RPS.
- `query-engine` можно масштабировать по глубине очереди NATS; все replicas
  используют durable JetStream consumer `query-workers` и делят jobs между собой.
- `meta-service` и `connection-manager` желательно масштабировать осторожно:
  discovery регистрирует адрес каждой replica через NATS, а registry
  Connection Manager должен видеть одинаковые plugin routes.
- Redis, PostgreSQL, NATS и S3 — отдельные отказоустойчивые зависимости; один
  Pod сервиса не превращает их в кластер.

В production нужны PodDisruptionBudget, requests/limits, NetworkPolicy,
TLS/mTLS между сервисами, отдельные ServiceAccount и Secret provider. Готовых
Deployment/Ingress/StatefulSet manifests в репозитории пока нет; добавленные
файлы описывают контракт конфигурации для их последующего создания.

## Web Admin

Web Admin — статический Vite bundle. `VITE_*` встраиваются на этапе сборки, а
не читаются из Kubernetes environment уже запущенного Nginx-контейнера:

```bash
cp services/web-admin/config.example.yaml /tmp/web-admin-config.yaml
# Сгенерируйте из него .env.production перед сборкой:
VITE_API_BASE_URL=https://api.example.com npm run build
```

Для изменения API URL после сборки нужен отдельный runtime-шаблон Nginx или
новая сборка образа.
