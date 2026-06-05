FROM golang:1.26-alpine3.23@sha256:f23e8b227fb4493eabe03bede4d5a32d04092da71962f1fb79b5f7d1e6c2a17f AS builder

WORKDIR /app

COPY . .

RUN apk add --no-cache make git \
    && make

FROM scratch AS runner

COPY --from=builder /app/healthcheck /healthcheck

ENTRYPOINT ["/healthcheck"]
