# syntax=docker/dockerfile:1

FROM golang:1.22-alpine AS build

ARG HTTP_PROXY=http://127.0.0.1:10808
ARG HTTPS_PROXY=http://127.0.0.1:10808
ARG NO_PROXY=localhost,127.0.0.1

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/sms-gateway ./cmd/api-gateway

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app app

WORKDIR /app
COPY --from=build /out/sms-gateway /app/sms-gateway

USER app
EXPOSE 8080

ENTRYPOINT ["/app/sms-gateway"]
