APP_NAME := routeflux
BUILD_DIR := bin

.PHONY: build test test-verbose coverage lint build-openwrt fmt

build:
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/routeflux

test:
	go test ./...

test-verbose:
	go test -v ./...

coverage:
	go test -coverprofile=coverage-parser.out ./internal/parser
	go tool cover -func=coverage-parser.out
	go test -coverprofile=coverage-probe.out ./internal/probe
	go tool cover -func=coverage-probe.out
	go test -coverpkg=./internal/backend/xray -coverprofile=coverage-backend.out ./internal/backend
	go tool cover -func=coverage-backend.out
	go test -coverprofile=coverage-store.out ./internal/store
	go tool cover -func=coverage-store.out

lint:
	go test ./...

fmt:
	go fmt ./...

build-openwrt:
	./scripts/build-openwrt.sh
