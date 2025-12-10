# See: https://gist.github.com/asukakenji/f15ba7e588ac42795f421b48b8aede63
# Note: these can be overridden on the command line e.g. `make PLATFORM=<platform> ARCH=<arch>`
PLATFORM=$(shell go env GOOS)
ARCH=$(shell go env GOARCH)

include .env
export

.PHONY: dev-backend
dev-backend:
	@echo "Running backend in development mode..."
	go run cmd/main.go;
