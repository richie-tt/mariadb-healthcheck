FROM golang:1.25-alpine3.23 AS builder

WORKDIR /app

COPY . .

RUN apk add --no-cache \
    make=4.4.1-r3 \
    git=2.52.0-r0 \
    && make

FROM scratch AS runner

COPY --from=builder /app/healthcheck /healthcheck

ENTRYPOINT ["/healthcheck"]
