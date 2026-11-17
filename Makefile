COMPOSE := docker compose -f deploy/local/docker-compose.yml
BUF     := go tool -modfile=tools/go.mod buf

# Local endpoints for the AWS SDK (ElasticMQ + DynamoDB Local).
export AWS_REGION ?= ap-southeast-1
export AWS_ACCESS_KEY_ID ?= local
export AWS_SECRET_ACCESS_KEY ?= local
export AWS_ENDPOINT_URL_SQS ?= http://localhost:9324
export AWS_ENDPOINT_URL_DYNAMODB ?= http://localhost:8000

.PHONY: up down logs test test-integration lint proto build run-api run-control run-worker run-reconciler

up: ## Start MySQL, SQS, DynamoDB, fake-market and run migrations
	$(COMPOSE) up -d --build --wait mysql sqs dynamodb fake-market
	$(COMPOSE) run --rm migrate
	$(COMPOSE) run --rm dynamodb-init

down:
	$(COMPOSE) down -v

logs:
	$(COMPOSE) logs -f

test:
	go test -race ./...

test-integration: ## Needs `make up`
	go test -race -tags=integration ./test/...

lint:
	golangci-lint run ./...

proto:
	$(BUF) lint
	$(BUF) generate

build:
	go build ./...

run-api:
	go run ./cmd/api

run-control:
	go run ./cmd/control

run-worker:
	go run ./cmd/worker

run-reconciler:
	go run ./cmd/reconciler
