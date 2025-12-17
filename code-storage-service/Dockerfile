FROM golang:1.24.3-alpine3.22 AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY shared/go.mod shared/go.sum ./shared/
COPY code-storage-service/go.mod code-storage-service/go.sum ./code-storage-service/
RUN cd code-storage-service && go mod download
COPY shared ./shared
COPY code-storage-service ./code-storage-service
RUN cd code-storage-service && go build -o /service ./cmd/main.go

FROM alpine:3.22
WORKDIR /app
COPY --from=builder /service .
COPY --from=builder /app/code-storage-service/config ./config
EXPOSE 8081
CMD ["./service"]
