ARG GOLANGCI_LINT_VERSION

FROM registry.access.redhat.com/ubi8/ubi AS builder

RUN yum install -y ca-certificates git go-toolset make
ENV PATH="/root/go/bin:${PATH}"
RUN curl -sfL https://password.corp.redhat.com/RH-IT-Root-CA.crt \
    -o /etc/pki/ca-trust/source/anchors/RH-IT-Root-CA.crt ;\ 
    update-ca-trust

RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin ${GOLANGCI_LINT_VERSION}

RUN go install github.com/golang/mock/mockgen@v1.6.0

###
FROM builder AS build-binary

COPY . /app
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

RUN make build-static

###
FROM registry.access.redhat.com/ubi8/ubi

COPY --from=build-binary /app/backplane-api /usr/local/bin/

EXPOSE 8001

ENTRYPOINT [ \
    "/usr/local/bin/backplane-api" \
]
