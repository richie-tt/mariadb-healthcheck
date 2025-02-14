FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY . .

RUN apk add --no-cache make=4.4.1-r2 \
    && make

FROM scratch AS runner

COPY --from=builder /app/healthcheck /healthcheck

ENTRYPOINT ["/healthcheck"]
