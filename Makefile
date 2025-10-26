unexport GOFLAGS

GOOS?=linux
GOARCH?=amd64
GOENV=GOOS=${GOOS} GOARCH=${GOARCH} CGO_ENABLED=0 GOFLAGS=
GOBUILDFLAGS=-gcflags="all=-trimpath=${GOPATH}" -asmflags="all=-trimpath=${GOPATH}"

RUN_IN_CONTAINER_CMD:=$(CONTAINER_ENGINE) run --platform linux/amd64 --rm -v $(shell pwd):/app -w=/app backplane-api-builder /bin/bash -c

OAPI_CODEGEN_VERSION=v2.5.0

generate-in-container:
	$(RUN_IN_CONTAINER_CMD) "make generate"

generate:
	$(shell mkdir -p pkg/client)
	go tool oapi-codegen -config oapi-codegen.yaml openapi/openapi.yaml
	go generate -v ./...
