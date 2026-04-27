FROM golang:1.25-alpine3.23@sha256:30e1078ea1ce91dcd8f48f27c0d7549cf23b32019de8b78cc0dc7b7707987dd5 AS builder

WORKDIR /app

COPY . .

RUN apk add --no-cache make git \
    && make

FROM scratch AS runner

COPY --from=builder /app/healthcheck /healthcheck

ENTRYPOINT ["/healthcheck"]
