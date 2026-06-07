SHELL := /bin/sh

WEB_DIR := web
APP_PKG := ./cmd/sim-server
IMAGE ?= shipsim:local
POSTGRES_TEST_PROJECT ?= shipsim-test
POSTGRES_TEST_DSN ?= postgres://shipsim_test:shipsim-test-only@127.0.0.1:15432/shipsim_test?sslmode=disable

.PHONY: dev test build lint docker-build postgres-test backend-dev frontend-dev frontend-install

dev:
	@echo "Starting backend on $${SHIP_SIM_ADDR:-:8080} and frontend on http://127.0.0.1:5173"
	@trap 'kill 0' INT TERM EXIT; go run $(APP_PKG) & cd $(WEB_DIR) && npm run dev

test:
	go test ./...
	cd $(WEB_DIR) && npm test

build:
	go build $(APP_PKG)
	cd $(WEB_DIR) && npm run build

lint:
	go vet ./...
	cd $(WEB_DIR) && npm run typecheck

docker-build:
	docker build -t $(IMAGE) .

postgres-test:
	@trap 'docker compose -p $(POSTGRES_TEST_PROJECT) -f docker-compose.test.yml --profile test down -v' EXIT; \
	docker compose -p $(POSTGRES_TEST_PROJECT) -f docker-compose.test.yml --profile test up -d db-test; \
	for i in $$(seq 1 60); do \
		if docker compose -p $(POSTGRES_TEST_PROJECT) -f docker-compose.test.yml exec -T db-test pg_isready -U shipsim_test -d shipsim_test >/dev/null 2>&1; then break; fi; \
		sleep 1; \
	done; \
	TEST_DATABASE_URL="$(POSTGRES_TEST_DSN)" go test -count=1 ./internal/store

backend-dev:
	go run $(APP_PKG)

frontend-dev:
	cd $(WEB_DIR) && npm run dev

frontend-install:
	cd $(WEB_DIR) && npm ci
