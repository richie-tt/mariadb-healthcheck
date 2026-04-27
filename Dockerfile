FROM golang:1.26-alpine3.23@sha256:f85330846cde1e57ca9ec309382da3b8e6ae3ab943d2739500e08c86393a21b1 AS builder

WORKDIR /app

COPY . .

RUN apk add --no-cache make git \
    && make

FROM scratch AS runner

COPY --from=builder /app/healthcheck /healthcheck

ENTRYPOINT ["/healthcheck"]
