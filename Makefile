SHELL := /bin/sh

WEB_DIR := web
APP_PKG := ./cmd/sim-server
IMAGE ?= shipsim:local

.PHONY: dev test build lint docker-build backend-dev frontend-dev frontend-install

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

backend-dev:
	go run $(APP_PKG)

frontend-dev:
	cd $(WEB_DIR) && npm run dev

frontend-install:
	cd $(WEB_DIR) && npm ci
