# syntax=docker/dockerfile:1

FROM golang:1.26 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o /metrics-service

FROM alpine:latest

COPY --from=builder /metrics-service /metrics-service

EXPOSE 8082

ENTRYPOINT ["/metrics-service"]
