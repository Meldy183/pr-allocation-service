FROM golang:1.24.3-alpine3.22 AS builder
WORKDIR /app
COPY go.mod go.mod
RUN go mod download
COPY . .
RUN go build -o /service ./cmd/pr-allocation-service/main.go

FROM alpine:3.22
WORKDIR /app
COPY --from=builder /service .
COPY --from=builder /app/config ./config
EXPOSE 8080
CMD ["./service"]

