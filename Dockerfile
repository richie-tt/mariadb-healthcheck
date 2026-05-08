FROM golang:1.26-alpine3.23@sha256:91eda9776261207ea25fd06b5b7fed8d397dd2c0a283e77f2ab6e91bfa71079d AS builder

WORKDIR /app

COPY . .

RUN apk add --no-cache make git \
    && make

FROM scratch AS runner

COPY --from=builder /app/healthcheck /healthcheck

ENTRYPOINT ["/healthcheck"]
