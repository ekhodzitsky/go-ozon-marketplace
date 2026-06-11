.PHONY: up down test test-integration test-e2e chaos-test proto lint migrate-user ci dev-up dev-down dev-seed dev-logs bench bench-grpc bench-graphql profile

up:
	docker compose -f infra/docker/docker-compose.yml up --build -d

down:
	docker compose -f infra/docker/docker-compose.yml down -v

dev-up:
	docker compose -f infra/docker/docker-compose.yml -f infra/docker/docker-compose.dev.yml up --build -d

dev-down:
	docker compose -f infra/docker/docker-compose.yml -f infra/docker/docker-compose.dev.yml down -v

dev-seed:
	go run scripts/seed.go

dev-logs:
	docker compose -f infra/docker/docker-compose.yml -f infra/docker/docker-compose.dev.yml logs -f

test:
	go test -race -count=1 ./...

test-integration:
	@for m in ./pkg ./services/* ./tests; do \
		echo "=== Integration tests $$m ==="; \
		(cd "$$m" && go test -race -count=1 -tags=integration ./...) || true; \
	done

test-e2e:
	cd tests && go test -race -tags=e2e ./e2e/...

chaos-test:
	cd tests && go test -tags=chaos -timeout=10m ./chaos/...

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

bench:
	bash tests/bench/grpc/bench.sh
	bash tests/bench/graphql/bench.sh

bench-grpc:
	bash tests/bench/grpc/bench.sh

bench-graphql:
	bash tests/bench/graphql/bench.sh

profile:
	bash scripts/profile.sh

ws-test:
	open docs/websocket-example.html
