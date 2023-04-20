unexport GOFLAGS

GOOS?=linux
GOARCH?=amd64
GOENV=GOOS=${GOOS} GOARCH=${GOARCH} CGO_ENABLED=0 GOFLAGS=
GOBUILDFLAGS=-gcflags="all=-trimpath=${GOPATH}" -asmflags="all=-trimpath=${GOPATH}"

RUN_IN_CONTAINER_CMD:=$(CONTAINER_ENGINE) run --platform linux/amd64 --rm -v $(shell pwd):/app -w=/app backplane-api-builder /bin/bash -c

OAPI_CODEGEN_VERSION=v1.12.4

generate-in-container:
	$(RUN_IN_CONTAINER_CMD) "make generate"

ensure-oapi-codegen:
	@ls $(GOPATH)/bin/oapi-codegen 1>/dev/null || go install github.com/deepmap/oapi-codegen/cmd/oapi-codegen@${OAPI_CODEGEN_VERSION}

generate: ensure-oapi-codegen
	$(shell mkdir -p pkg/client)
	oapi-codegen -package Openapi -generate types,client,spec openapi/openapi.yaml > pkg/client/BackplaneApi.go
	go generate -v ./...
