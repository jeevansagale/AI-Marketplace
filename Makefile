.PHONY: dev demo test fmt vet up down migrate seed build-web build-go lint

COMPOSE := docker compose -f infra/docker-compose/dev.yml

dev: up
	@echo "Gateway :8080 | ControlPlane :8081 | Web :3000"

up:
	$(COMPOSE) up -d --build
	@$(MAKE) migrate seed

demo: up run-web

run-web:
	cd apps/web && npm run dev

down:
	$(COMPOSE) down

migrate:
	$(COMPOSE) exec -T db psql -U marketplace -d marketplace -f /docker-entrypoint-initdb.d/0001_init.sql || true

seed:
	$(COMPOSE) exec -T db psql -U marketplace -d marketplace -c "SELECT count(*) FROM models;" 2>/dev/null || true

test:
	cd apps/gateway && go test ./...
	cd apps/controlplane && go test ./...

lint:
	cd apps/gateway && golangci-lint run ./...
	cd apps/controlplane && golangci-lint run ./...

fmt:
	cd apps/gateway && gofmt -w .
	cd apps/controlplane && gofmt -w .

vet:
	cd apps/gateway && go vet ./...
	cd apps/controlplane && go vet ./...

build-web:
	cd apps/web && npm run build

build-go:
	cd apps/gateway && go build ./...
	cd apps/controlplane && go build ./...