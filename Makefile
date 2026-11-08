BINARY := bin/protoc-gen-frankenphp-go-grpc
FRANKENPHP_BUILDER_IMAGE ?= dunglas/frankenphp:1.13.4-builder

##
## protoc-gen-frankenphp-go-grpc
## -------
##
build:					## Builds the binary
	go build -o $(BINARY) ./cmd/protoc-gen-frankenphp-go-grpc

test:					## Tests the generator without native PHP libraries
	go test ./cmd/... ./internal/...

fmt:					## Runs gofmt
	gofmt -w $$(find . -name '*.go' -not -path './testdata/generated/*')

vet:					## Vets the generator without native PHP libraries
	go vet ./cmd/... ./internal/...

install:				## Installs protoc-gen-frankenphp-go-grpc
	go install ./cmd/protoc-gen-frankenphp-go-grpc

tools:					## Installs protoc-gen-go, protoc-gen-go-grpc
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

generate: build			## Generates files for the test proto fixtures
	mkdir -p testdata/generated
	PATH="$$PWD/bin:$$PATH" protoc -I testdata/proto \
		--go_out=testdata/generated --go_opt=paths=source_relative \
		--go-grpc_out=testdata/generated --go-grpc_opt=paths=source_relative \
		--frankenphp-go-grpc_out=testdata/generated --frankenphp-go-grpc_opt=paths=source_relative \
		testdata/proto/server-test.proto testdata/proto/second-service-test.proto

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

##
## runtime
## -------
##

# These targets require the FrankenPHP native development environment.
test-runtime:				## Tests the runtime (requires native FrankenPHP libraries)
	go test ./runtime

vet-runtime:				## Vets the runtime (requires native FrankenPHP libraries)
	go vet ./runtime

check-runtime-docker:		## Tests and vets the runtime in the FrankenPHP builder container
	docker run --rm --entrypoint bash \
		-v "$(CURDIR):/workspace:ro" -w /workspace \
		$(FRANKENPHP_BUILDER_IMAGE) \
		-c 'export CGO_CFLAGS="$${CGO_CFLAGS:-} $$(php-config --includes)"; make test-runtime vet-runtime'

.PHONY: test-runtime vet-runtime check-runtime-docker
