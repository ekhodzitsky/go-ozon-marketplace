ARG SERVICE_NAME
FROM golang:1.26-alpine AS builder
ARG SERVICE_NAME

WORKDIR /app

COPY services/${SERVICE_NAME}/go.mod services/${SERVICE_NAME}/go.sum ./services/${SERVICE_NAME}/
COPY api/go.mod api/go.sum ./api/
COPY pkg/go.mod pkg/go.sum ./pkg/

RUN printf 'go 1.26\n\nuse (\n\t./api\n\t./pkg\n\t./services/%s\n)\n' "${SERVICE_NAME}" > /app/go.work

RUN go work sync
WORKDIR /app/services/${SERVICE_NAME}
RUN go mod download

COPY api/ /app/api/
COPY pkg/ /app/pkg/
COPY services/${SERVICE_NAME}/ /app/services/${SERVICE_NAME}/

RUN CGO_ENABLED=0 GOOS=linux go build \
    -buildvcs=false \
    -ldflags="-w -s -extldflags '-static'" \
    -trimpath \
    -o /bin/server ./cmd/main.go

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /bin/server /server
USER 65532:65532
ENTRYPOINT ["/server"]
