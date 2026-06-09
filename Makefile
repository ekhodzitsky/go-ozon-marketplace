.PHONY: up down test proto lint migrate-user ci

up:
	docker compose -f infra/docker/docker-compose.yml up --build -d

down:
	docker compose -f infra/docker/docker-compose.yml down -v

test:
	go test -race -count=1 ./...

proto:
	cd api && buf generate

lint:
	golangci-lint run ./...

ci: lint test

migrate-user:
	migrate -path services/user-service/migrations -database "$(USER_DB_URL)" up

migrate-%:
	migrate -path services/$*/migrations -database "$(DB_URL)" up
