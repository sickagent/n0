FROM golang:1.26-alpine AS builder
WORKDIR /build
COPY go.work go.work.sum* ./
COPY proto proto
COPY pkg/shared pkg/shared
COPY services/agent-gateway services/agent-gateway
COPY services/connection-manager services/connection-manager
COPY services/meta-service services/meta-service
COPY services/query-engine services/query-engine
RUN --mount=type=cache,target=/go/pkg/mod \
    go work sync && cd services/agent-gateway && CGO_ENABLED=0 go build -o /agent-gateway ./cmd

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /agent-gateway /agent-gateway
USER nonroot:nonroot
EXPOSE 8080 8081 9090
ENTRYPOINT ["/agent-gateway"]
