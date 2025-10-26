FROM golang:1.24

RUN go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.0
