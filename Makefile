.PHONY: up down test test-integration test-e2e chaos-test proto lint migrate-user ci dev-up dev-down dev-seed dev-logs bench bench-grpc bench-graphql profile build \
	build-analytics-service build-api-gateway build-catalog-service build-inventory-service build-notification-service build-order-service build-payment-service build-user-service \
	build-order

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

MODULES := ./api ./pkg ./scripts ./services/* ./tests

test:
	@for m in $(MODULES); do \
		echo "=== Testing $$m ==="; \
		(cd "$$m" && go test -race -count=1 ./...) || exit 1; \
	done

test-integration:
	@for m in $(MODULES); do \
		echo "=== Integration tests $$m ==="; \
		(cd "$$m" && go test -p 1 -race -count=1 -tags=integration ./...) || exit 1; \
	done

test-e2e:
	cd tests && go test -race -tags=e2e ./e2e/...

chaos-test:
	cd tests && go test -tags=chaos -timeout=10m ./chaos/...

proto:
	cd api && buf generate

lint:
	@for m in $(MODULES); do \
		echo "=== Linting $$m ==="; \
		(cd "$$m" && golangci-lint run ./...) || exit 1; \
	done

coverage:
	@set -e; \
	if ! command -v gocovmerge >/dev/null 2>&1; then \
		echo "Installing gocovmerge..."; \
		go install github.com/wadey/gocovmerge@latest; \
	fi; \
	coverage_files=(); \
	for m in $(MODULES); do \
		echo "=== Coverage $$m ==="; \
		covfile="coverage_$$(echo "$$m" | tr '/' '_').out"; \
		cd "$$m" && go test -race -count=1 -coverprofile="$$covfile" -covermode=atomic ./...; \
		cd - > /dev/null; \
		if [ -f "$$m/$$covfile" ]; then \
			coverage_files+=("$$m/$$covfile"); \
		fi; \
	done; \
	if [ $${#coverage_files[@]} -gt 0 ]; then \
		gocovmerge "$${coverage_files[@]}" > coverage.out; \
		go tool cover -func=coverage.out | grep total; \
	fi

ci: lint test coverage

migrate-user:
	migrate -path services/user-service/migrations -database "$(POSTGRES_DSN)" up

migrate-%:
	migrate -path services/$*/migrations -database "$(POSTGRES_DSN)" up

SERVICES := analytics-service api-gateway catalog-service inventory-service notification-service order-service payment-service user-service

build: $(addprefix build-,$(SERVICES))

$(addprefix build-,$(SERVICES)):
	@mkdir -p bin
	@svc=$(patsubst build-%,%,$@); \
	echo "Building $$svc -> bin/$$svc"; \
	cd services/$$svc && CGO_ENABLED=0 go build -ldflags="-w -s" -trimpath -o ../../bin/$$svc ./cmd/main.go

build-order: build-order-service

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
