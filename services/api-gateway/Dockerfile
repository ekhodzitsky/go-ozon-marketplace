# syntax=docker/dockerfile:1

# Этап сборки: компилируем Rust-бинарник api-gateway.
FROM rust:1.88-bookworm AS builder

WORKDIR /app

# Устанавливаем buf (нужен build.rs для экспорта proto), protoc и базовые зависимости.
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
       curl ca-certificates git protobuf-compiler libprotobuf-dev pkg-config libssl-dev \
    && rm -rf /var/lib/apt/lists/* \
    && BUF_VERSION=1.47.0 \
    && curl -sSL "https://github.com/bufbuild/buf/releases/download/v${BUF_VERSION}/buf-$(uname -s)-$(uname -m)" -o /usr/local/bin/buf \
    && chmod +x /usr/local/bin/buf

# Копируем proto-контракты и исходники сервиса.
COPY api/ /app/api/
COPY services/api-gateway/ /app/services/api-gateway/

WORKDIR /app/services/api-gateway
RUN cargo build --release

# Этап исполнения: минимальный образ с nonroot-пользователем.
FROM debian:bookworm-slim AS runtime

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 65532 nonroot \
    && useradd --uid 65532 --gid nonroot --shell /usr/sbin/nologin nonroot

WORKDIR /app
COPY --from=builder /app/services/api-gateway/target/release/api-gateway /app/api-gateway

USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/api-gateway"]
