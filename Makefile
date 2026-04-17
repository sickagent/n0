# Enterprise Makefile for n0
.SHELLFLAGS = -ec
.SILENT:

GO := go
DOCKER := docker
COMPOSE := docker compose
PROTOC_IMAGE := namely/protoc-all:1.53_2

SERVICES := services/agent-gateway services/meta-service services/query-engine services/connection-manager

.PHONY: all
all: build

.PHONY: proto
proto:
	@echo "Generating protobuf Go code..."
	@mkdir -p proto/gen/go
	@protoc \
		--go_out=proto/gen/go \
		--go_opt=paths=source_relative \
		--go-grpc_out=proto/gen/go \
		--go-grpc_opt=paths=source_relative \
		-I proto \
		proto/lensagent/v1/*.proto
	@echo "Done."

.PHONY: tidy
tidy:
	@echo "Tidening Go modules..."
	$(GO) work sync
	@for svc in $(SERVICES); do \
		cd $$svc && $(GO) mod tidy && cd ../..; \
	done
	cd pkg/shared && $(GO) mod tidy && cd ../..

.PHONY: build
build:
	@echo "Building services..."
	@for svc in $(SERVICES); do \
		echo "  -> $$svc"; \
		cd $$svc && CGO_ENABLED=0 $(GO) build -o ../../bin/$$(basename $$svc) ./cmd && cd ../..; \
	done

.PHONY: docker-build
docker-build:
	@echo "Building Docker images..."
	@for svc in $(SERVICES); do \
		name=$$(basename $$svc); \
		echo "  -> $$name"; \
		$(DOCKER) build -t n0/$$name:latest -f $$svc/Dockerfile .; \
	done

.PHONY: up
up:
	$(COMPOSE) -f deployments/docker-compose.yml up -d

.PHONY: down
down:
	$(COMPOSE) -f deployments/docker-compose.yml down

.PHONY: lint
lint:
	@echo "Running golangci-lint..."
	@for svc in $(SERVICES); do \
		cd $$svc && golangci-lint run ./... && cd ../..; \
	done

.PHONY: test
test:
	@echo "Running tests..."
	@for svc in $(SERVICES); do \
		cd $$svc && $(GO) test ./... && cd ../..; \
	done

.PHONY: migrate-up
migrate-up:
	@echo "Running meta-service migrations..."
	$(DOCKER) run --rm -v $(PWD)/services/meta-service/migrations:/migrations \
		migrate/migrate:v4.17.0 \
		-path=/migrations -database "postgres://postgres:postgres@localhost:5432/meta?sslmode=disable" up

.PHONY: migrate-create
migrate-create:
	@read -p "Migration name: " name; \
	migrate create -ext sql -dir services/meta-service/migrations -seq $$name
