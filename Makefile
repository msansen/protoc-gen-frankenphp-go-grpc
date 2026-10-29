BINARY := bin/protoc-gen-frankenphp-go-grpc

##
## protoc-gen-frankenphp-go-grpc
## -------
##
build:					## Builds the binary
	go build -o $(BINARY) ./cmd/protoc-gen-frankenphp-go-grpc

test:					## Runs the test files
	go test ./...

fmt:					## Runs gofmt
	gofmt -w $$(find . -name '*.go' -not -path './testdata/generated/*')

vet:					## Runs go vet
	go vet ./...

install:				## Installs protoc-gen-frankenphp-go-grpc
	go install ./cmd/protoc-gen-frankenphp-go-grpc

tools:					## Installs protoc-gen-go, protoc-gen-go-grpc
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

generate: build			## Generates files for testdata/proto/server-test.proto
	mkdir -p testdata/generated
	PATH="$$PWD/bin:$$PATH" protoc -I testdata/proto \
		--go_out=testdata/generated --go_opt=paths=source_relative \
		--go-grpc_out=testdata/generated --go-grpc_opt=paths=source_relative \
		--frankenphp-go-grpc_out=testdata/generated --frankenphp-go-grpc_opt=paths=source_relative \
		testdata/proto/server-test.proto

docker-build:			## Builds docker image locally
	docker build --target binary -t protoc-gen-frankenphp-go-grpc:local .

check: fmt test vet		## Runs gofmt, tests and go vet

clean:					## Cleans the generated files
	@rm -rf bin testdata/generated || true
	@rm -rf bin generated/* || true

.PHONY: build test fmt vet install tools generate check clean docker-build

.DEFAULT_GOAL:= help
help:
	@grep -Eh '(^[a-zA-Z_-]+:.*?##.*$$)|(^##)' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[32m%-30s\033[0m %s\n", $$1, $$2}' | sed -e 's/\[32m##/[33m/'

.PHONY: help
