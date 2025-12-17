FROM golang:1.24.3-alpine3.22 AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY shared/go.mod shared/go.sum ./shared/
COPY pr-allocation-service/go.mod pr-allocation-service/go.sum ./pr-allocation-service/
RUN cd pr-allocation-service && go mod download
COPY shared ./shared
COPY pr-allocation-service ./pr-allocation-service
RUN cd pr-allocation-service && go build -o /service ./cmd/pr-allocation-service/main.go

FROM alpine:3.22
WORKDIR /app
COPY --from=builder /service .
COPY --from=builder /app/pr-allocation-service/config ./config
EXPOSE 8080
CMD ["./service"]
