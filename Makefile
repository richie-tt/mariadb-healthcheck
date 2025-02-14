.DEFAULT_GOAL := all

APPLICATION?=healthcheck

VERSION?=$(shell git describe --tags 2>/dev/null || echo 0.0.0)
COMMITSHA?=$(shell git rev-parse --short HEAD)
BUILD_TIME?=$(shell date '+%Y-%m-%d_%H:%M:%S')

GOOS?=$(shell uname | tr 'A-Z' 'a-z')
GOARCH?=amd64

.PHONY: all
all: clean deps build


.PHONY: build
build:
	$(info Building for ${GOOS}/${GOARCH})
	CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build \
		-ldflags "-s -w \
		-X main.Version=${VERSION} \
		-X main.Commit=${COMMITSHA} \
		-X main.BuildDate=${BUILD_TIME}" \
		-o ${APPLICATION} $(CURDIR)/cmd/${APPLICATION}

.PHONY: deps
deps:
	$(info Getting dependencies)
	go work sync

.PHONY: clean
clean:
	$(info Cleaning GO build)
	go clean

.PHONY: test
test:
	$(info Running tests)
	go test -v .
