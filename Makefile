unexport GOFLAGS

GOOS?=linux
GOARCH?=amd64
GOENV=GOOS=${GOOS} GOARCH=${GOARCH} CGO_ENABLED=0 GOFLAGS=
GOBUILDFLAGS=-gcflags="all=-trimpath=${GOPATH}" -asmflags="all=-trimpath=${GOPATH}"

CONTAINER_ENGINE:=$(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null)
RUN_IN_CONTAINER_CMD:=$(CONTAINER_ENGINE) run --platform linux/amd64 --rm -v $(shell pwd):/app -w=/app backplane-api-builder /bin/bash -c

openapi-image:
	$(CONTAINER_ENGINE) build --pull --platform linux/amd64 -f openapi.Dockerfile -t backplane-api-openapi .

generate-in-container:
	$(RUN_IN_CONTAINER_CMD) "make generate"

generate: openapi-image
	$(CONTAINER_ENGINE) run --platform linux/amd64 --privileged=true --rm -v $(shell pwd):/app backplane-api-openapi /bin/sh -c "mkdir -p /app/pkg/client && oapi-codegen -generate types,client,spec /app/openapi/openapi.yaml > /app/pkg/client/BackplaneApi.go"
