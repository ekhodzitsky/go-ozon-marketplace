.PHONY: up down test test-integration test-e2e proto lint migrate-user ci

up:
	docker compose -f infra/docker/docker-compose.yml up --build -d

down:
	docker compose -f infra/docker/docker-compose.yml down -v

test:
	go test -race -count=1 ./...

test-integration:
	@for m in ./pkg ./services/* ./tests; do \
		echo "=== Integration tests $$m ==="; \
		(cd "$$m" && go test -race -count=1 -tags=integration ./...) || true; \
	done

test-e2e:
	cd tests && go test -race -tags=e2e ./e2e/...

proto:
	cd api && buf generate

lint:
	golangci-lint run ./...

coverage:
	go test -race -count=1 -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out | grep total

ci: lint test coverage

migrate-user:
	migrate -path services/user-service/migrations -database "$(USER_DB_URL)" up

migrate-%:
	migrate -path services/$*/migrations -database "$(DB_URL)" up
