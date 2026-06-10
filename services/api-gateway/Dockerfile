FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.work go.work.sum ./
COPY services/api-gateway/go.mod services/api-gateway/go.sum ./services/api-gateway/
COPY api/go.mod api/go.sum ./api/
COPY pkg/go.mod pkg/go.sum ./pkg/

RUN printf 'go 1.26\n\nuse (\n\t./api\n\t./pkg\n\t./services/api-gateway\n)\n' > /app/go.work

RUN go work sync
WORKDIR /app/services/api-gateway
RUN go mod download

COPY api/ /app/api/
COPY pkg/ /app/pkg/
COPY services/api-gateway/ /app/services/api-gateway/

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -extldflags '-static'" \
    -trimpath \
    -o /bin/server ./cmd/main.go

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /bin/server /server
USER 65532:65532
ENTRYPOINT ["/server"]
