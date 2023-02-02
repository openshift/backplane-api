unexport GOFLAGS

GOOS?=linux
GOARCH?=amd64
GOENV=GOOS=${GOOS} GOARCH=${GOARCH} CGO_ENABLED=0 GOFLAGS=
GOBUILDFLAGS=-gcflags="all=-trimpath=${GOPATH}" -asmflags="all=-trimpath=${GOPATH}"

IMAGE_REGISTRY?=quay.io
IMAGE_REPOSITORY?=app-sre
IMAGE_NAME?=backplane-api
VERSION=$(shell git rev-parse --short=7 HEAD)
UNAME_S := $(shell uname -s)

IMAGE_URI_VERSION:=$(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(IMAGE_NAME):$(VERSION)
IMAGE_URI_LATEST:=$(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(IMAGE_NAME):latest

HOME=$(shell mktemp -d)
GOLANGCI_LINT_VERSION=v1.50.1

CONTAINER_ENGINE:=$(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null)
RUN_IN_CONTAINER_CMD:=$(CONTAINER_ENGINE) run --platform linux/amd64 --rm -v $(shell pwd):/app -w=/app backplane-api-builder /bin/bash -c

container-all: test-in-container build-in-container lint-in-container image
	@echo Ran all container deps

build-in-container: clean build-image
	$(RUN_IN_CONTAINER_CMD) "make build-static"

build-static: clean
	go build -a -installsuffix cgo -ldflags '-extldflags \"-static\"' -o backplane-api

build: clean
	go build -o backplane-api

.PHONY: clean
clean:
	rm -f backplane-api

build-image:
	$(CONTAINER_ENGINE) build --pull --platform linux/amd64 --build-arg=GOLANGCI_LINT_VERSION -t backplane-api-builder --target builder .

openapi-image:
	$(CONTAINER_ENGINE) build --pull --platform linux/amd64 -f openapi.Dockerfile -t backplane-api-openapi .

.PHONY: lint-in-container
lint-in-container: build-image
	$(RUN_IN_CONTAINER_CMD) "go mod download && make lint"

# Installed using instructions from: https://golangci-lint.run/usage/install/#linux-and-windows
getlint:
	@mkdir -p $(GOPATH)/bin
	@ls $(GOPATH)/bin/golangci-lint 1>/dev/null || (echo "Installing golangci-lint..." && curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOPATH)/bin $(GOLANGCI_LINT_VERSION))

.PHONY: lint
lint: getlint
	$(GOPATH)/bin/golangci-lint run
	
test-in-container: build-image
	$(RUN_IN_CONTAINER_CMD) "make test"

test:
	go test -v ./...
	@echo Run \'make test-cover\' to see the test coverage report

test-cover:
	go test -cover -coverprofile=coverage.out ./...

cover-html:
	go tool cover -html=coverage.out

generate-in-container: build-image
	$(RUN_IN_CONTAINER_CMD) "make generate"

generate: openapi-image
	$(CONTAINER_ENGINE) run --platform linux/amd64 --privileged=true --rm -v $(shell pwd):/app backplane-api-openapi /bin/sh -c "mkdir -p /app/pkg/client && oapi-codegen -generate types,client,spec /app/openapi/openapi.yaml > /app/pkg/client/BackplaneApi.go"
	go generate -v ./...

dev-certs:
	bash -c 'openssl req \
		-x509 \
		-out localhost.crt \
		-keyout localhost.key \
		-newkey rsa:4096 \
		-nodes \
		-sha256 \
		-subj "/CN=localhost" \
		-extensions san \
		-config <( \
			echo "[req]"; \
			echo "distinguished_name=req"; \
			echo "[san]"; \
			echo "subjectAltName=DNS:localhost")'
ifeq ($(UNAME_S),Darwin)
	security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain localhost.crt
endif
ifeq ($(UNAME_S),Linux)
	cp localhost.crt /etc/pki/tls/certs/
	cp localhost.crt /etc/pki/ca-trust/source/anchors/
	update-ca-trust extract
endif

RUN_ARGS=

run-local: build
	USE_RH_API=false ./backplane-api  \
		$(RUN_ARGS) \
		-jwks-file ./configs/jwks.json \
		-jwks-url https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/certs \
		-acl-file ./configs/acl.yml \
		-ocm-config ./configs/ocm.json \
		-roles-file ./configs/roles.yml \
		--enable-https=true \
		--https-cert-file=localhost.crt \
		--https-key-file=localhost.key \
		-v 8

run-local-with-cloud: RUN_ARGS=--cloud-config ./configs/cloud-config.yml
run-local-with-cloud: run-local


image:
	$(CONTAINER_ENGINE) build -t $(IMAGE_URI_VERSION) .
	$(CONTAINER_ENGINE) tag $(IMAGE_URI_VERSION) $(IMAGE_URI_LATEST)

push: image
	$(CONTAINER_ENGINE) push $(IMAGE_URI_VERSION)
	$(CONTAINER_ENGINE) push $(IMAGE_URI_LATEST)

skopeo-push: image
	skopeo copy \
		--dest-creds "${QUAY_USER}:${QUAY_TOKEN}" \
		"docker-daemon:${IMAGE_URI_VERSION}" \
		"docker://${IMAGE_URI_VERSION}"
	skopeo copy \
		--dest-creds "${QUAY_USER}:${QUAY_TOKEN}" \
		"docker-daemon:${IMAGE_URI_LATEST}" \
		"docker://${IMAGE_URI_LATEST}"
