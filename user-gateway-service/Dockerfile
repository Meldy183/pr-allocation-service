FROM golang:1.24.3-alpine3.22 AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY shared/go.mod shared/go.sum ./shared/
COPY pr-allocation-service/go.mod pr-allocation-service/go.sum ./pr-allocation-service/
COPY code-storage-service/go.mod code-storage-service/go.sum ./code-storage-service/
COPY user-gateway-service/go.mod user-gateway-service/go.sum ./user-gateway-service/
RUN cd user-gateway-service && go mod download
COPY shared ./shared
COPY user-gateway-service ./user-gateway-service
RUN cd user-gateway-service && go build -o /service ./cmd/main.go

FROM alpine:3.22
WORKDIR /app
COPY --from=builder /service .
COPY --from=builder /app/user-gateway-service/config ./config
EXPOSE 8082
CMD ["./service"]

