# protoc-gen-frankenphp-go-grpc

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Generate the Go bridge between your protobuf services and PHP handlers running with [FrankenPHP gRPC](https://github.com/dunglas/frankenphp-grpc).

`protoc-gen-frankenphp-go-grpc` is a `protoc` plugin. It generates gRPC server implementations, message converters, and registration helpers so you can implement service methods in PHP, with optional native Go overrides.

> **Experimental — provided as is.** APIs, generated code, and the Go/PHP exchange format may change without notice. **Streaming support is provisional and its design will evolve.** This project is not currently recommended for production use. Pin your dependencies and validate the generated code against your application before upgrading.

## Place in the ecosystem

- [dunglas/frankenphp-grpc](https://github.com/dunglas/frankenphp-grpc) provides the FrankenPHP extension that runs a gRPC server and dispatches work to PHP. It is also experimental.
- **This plugin** generates the Go integration code from your `.proto` definitions. It complements `protoc-gen-go` and `protoc-gen-go-grpc`.
- [msansen/frankenphp-grpc-bundle](https://github.com/msansen/frankenphp-grpc-bundle) is the recommended integration path for Symfony applications. Follow its documentation for Symfony-specific installation and setup; the examples below describe the underlying Go/PHP bridge.

The generated code belongs in a Go module included in your custom FrankenPHP build, alongside `frankenphp-grpc`. Installing this generator alone does not start a server or install a PHP runtime extension.

The request flow is:

```text
gRPC client → generated Go server → PHP handler → generated Go response decoder → gRPC client
```

A native Go override handles the selected method directly, without dispatching it to PHP.

## Requirements

For generation:

- `protoc` on your `PATH`.
- `protoc-gen-go` and `protoc-gen-go-grpc` on your `PATH`.
- This plugin on your `PATH`, or explicitly selected with `protoc --plugin`.
- A `go_package` option in your protobuf definitions.

Building this plugin from source requires Go 1.24 or later. Running the generated server requires a custom FrankenPHP build with the gRPC extension, your generated package, and a configured PHP worker. Follow the [FrankenPHP extension documentation](https://frankenphp.dev/docs/extensions/) and the [frankenphp-grpc setup instructions](https://github.com/dunglas/frankenphp-grpc).

Install the companion generators if needed:

```sh
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

For reproducible builds, replace `@latest` with explicit versions and record the versions of all generators and runtime dependencies you use together.

## Installation

### From source

```sh
go install github.com/msansen/protoc-gen-frankenphp-go-grpc/cmd/protoc-gen-frankenphp-go-grpc@latest
```

Ensure `$(go env GOPATH)/bin` is on your `PATH`.

### Release binaries

Download the asset for your operating system and architecture from [GitHub Releases](https://github.com/msansen/protoc-gen-frankenphp-go-grpc/releases), together with `checksums.txt` to verify the download.

Binary names follow `protoc-gen-frankenphp-go-grpc-<os>-<arch>`, with `.exe` on Windows. Rename the downloaded executable to `protoc-gen-frankenphp-go-grpc` (or `protoc-gen-frankenphp-go-grpc.exe` on Windows) and place it on your `PATH`. On Unix, make it executable with `chmod +x`.

Alternatively, provide its location explicitly:

```sh
protoc --plugin=protoc-gen-frankenphp-go-grpc=/absolute/path/to/plugin \
  --frankenphp-go-grpc_out=generated \
  -I proto proto/greeter.proto
```

### Container image as a binary distribution

The image `ghcr.io/msansen/protoc-gen-frankenphp-go-grpc` packages the plugin executable for use in build environments. It is a distribution artifact: copy the binary into your toolchain or build stage, then let `protoc` execute it there.

For example, in a Dockerfile whose build stage already contains `protoc` and the companion generators:

```dockerfile
# Replace <release-tag> with an available image tag.
COPY --from=ghcr.io/msansen/protoc-gen-frankenphp-go-grpc:<release-tag> \
    /protoc-gen-frankenphp-go-grpc /usr/local/bin/protoc-gen-frankenphp-go-grpc
```

Choose an image architecture matching the environment that executes the generator. The image is not a FrankenPHP runtime image.

## Generate and register a service

### 1. Define the service

In a Go module named `example.com/myapp`, create `proto/greeter.proto`:

```proto
syntax = "proto3";

package example.v1;
option go_package = "example.com/myapp/generated;pb";

service Greeter {
  rpc SayHello(HelloRequest) returns (HelloReply);
}

message HelloRequest {
  string name = 1;
}

message HelloReply {
  string message = 1;
}
```

### 2. Generate the Go files

Run from the module root:

```sh
mkdir -p generated
protoc -I proto \
  --go_out=generated --go_opt=paths=source_relative \
  --go-grpc_out=generated --go-grpc_opt=paths=source_relative \
  --frankenphp-go-grpc_out=generated --frankenphp-go-grpc_opt=paths=source_relative \
  proto/greeter.proto
```

This produces:

| File | Purpose |
| --- | --- |
| `greeter.pb.go` | Protobuf message types, generated by `protoc-gen-go`. |
| `greeter_grpc.pb.go` | gRPC service interfaces, generated by `protoc-gen-go-grpc`. |
| `greeter_frankenphp_grpc.pb.go` | Server, PHP bridge, and message converters, generated by this plugin. |

Only protobuf files containing services produce a bridge file. Keep application code outside generated files so regeneration does not overwrite it.

### 3. Register the generated server

Create a handwritten Go package:

```go
package grpcserver

import (
    phpgrpc "github.com/dunglas/frankenphp-grpc"
    pb "example.com/myapp/generated"
    "google.golang.org/grpc"
)

func init() {
    phpgrpc.RegisterGrpcServerFactory(func() *grpc.Server {
        server := grpc.NewServer()
        pb.RegisterGreeterFrankenPHPServer(server, pb.GreeterFrankenPHPOptions{})
        return server
    })
}
```

Include this package in your custom FrankenPHP build so its `init` function runs. Configure the gRPC listener and PHP worker following the extension's [setup instructions](https://github.com/dunglas/frankenphp-grpc). This registration snippet is one part of that setup, not a standalone server program.

### 4. Handle requests in PHP

Choose the Symfony integration below or implement a PHP worker callback directly.

#### Symfony applications

Use [msansen/frankenphp-grpc-bundle](https://github.com/msansen/frankenphp-grpc-bundle) to route RPCs to Symfony services. After installing and registering the bundle, enable service autowiring and autoconfiguration and create `src/Grpc/Handler/GreeterHandler.php`:

```php
<?php

namespace App\Grpc\Handler;

use FrankenPhpGrpcBundle\Attribute\GrpcHandler;

#[GrpcHandler('/example.v1.Greeter/SayHello')]
final class GreeterHandler
{
    public function __invoke(array $request): array
    {
        return ['message' => 'Hello, ' . $request['name']];
    }
}
```

The Symfony handler receives only the RPC payload and returns the response payload. The bundle's `GrpcBridge` handles request envelopes, routing through the dispatcher, response envelopes, and exception mapping. Constructor dependencies are injected by Symfony.

Use the bundle's [gRPC worker example](https://github.com/msansen/frankenphp-grpc-bundle/blob/main/examples/grpc-worker.php), which boots the Symfony kernel and delegates worker callbacks to `GrpcBridge::handle()`. Keep `DisableEnvelope` at its default value of `false`: the bundle requires the [versioned bridge protocol](https://github.com/msansen/frankenphp-grpc-bundle/blob/main/docs/bridge-protocol.md). Follow the bundle's [installation and configuration instructions](https://github.com/msansen/frankenphp-grpc-bundle#installation) for the complete Symfony setup.

#### PHP worker callback without the bundle

By default, the generated Go bridge sends a versioned envelope to the PHP worker callback:

```php
[
    '__frankenphp_grpc' => [
        'method' => '/example.v1.Greeter/SayHello',
        'version' => 1,
        'payload' => ['name' => 'World'],
    ],
]
```

When implementing this callback yourself, return a response envelope. For example, the callback passed to your worker's `frankenphp_handle_request()` loop can be:

```php
$handler = static function (array $request): array {
    $envelope = $request['__frankenphp_grpc'];

    if ($envelope['version'] !== 1) {
        throw new \RuntimeException('Unsupported bridge version');
    }

    if ($envelope['method'] !== '/example.v1.Greeter/SayHello') {
        return [
            '__frankenphp_grpc' => [
                'version' => 1,
                'ok' => false,
                'error' => ['code' => 12, 'message' => 'Method not implemented'],
            ],
        ];
    }

    return [
        '__frankenphp_grpc' => [
            'version' => 1,
            'ok' => true,
            'payload' => [
                'message' => 'Hello, ' . $envelope['payload']['name'],
            ],
        ],
    ];
};
```

This defines the callback only; the extension's worker loop and runtime configuration are still required.

## Go/PHP exchange format

The envelope version is currently `1`. The method is the fully qualified gRPC method path, including the leading slash.

Successful responses require `version`, `ok: true`, and a `payload`. Error responses require `version`, `ok: false`, and an `error` containing a numeric gRPC status code from `1` to `16` and a string `message`. For example, `3` is `InvalidArgument` and `12` is `Unimplemented`.

Malformed response envelopes and unsupported envelope versions return gRPC `Internal`. Existing gRPC status errors from the bridge are preserved; other PHP bridge errors become `Internal` with the message `PHP handler failed`. Message decoding failures currently return ordinary Go errors, so they do not have the same explicit status mapping.

With the Symfony bundle, handlers can throw `GrpcStatusException` to return an explicit status, or use custom exception mappers for application errors. `GrpcBridge` converts these exceptions into error envelopes that the generated Go code decodes. Unhandled exceptions become `Internal` with the message `Internal server error`. See the bundle's [error handling documentation](https://github.com/msansen/frankenphp-grpc-bundle#error-handling).

### Message values

The converters use protobuf field names, such as `user_id`, rather than Go field names or protobuf JSON names. The payload is an array representation of the message, not protobuf JSON.

| Protobuf value | PHP-facing representation |
| --- | --- |
| Message | Associative array of fields. |
| Repeated field | List of values. |
| Map | Associative array; non-string keys are encoded as strings. |
| Enum | Numeric enum value. |
| `bytes` | Binary string, without base64 encoding. |
| `uint64`, `fixed64` | Decimal string on Go-to-PHP encoding, to preserve the unsigned range. |
| Other scalar | Corresponding string, boolean, integer, or float. |

Absent optional scalar fields and absent singular messages are omitted during encoding. A oneof emits only its selected member. Missing fields in a PHP response retain their Go zero value in the newly allocated response.

Converters are still being developed. See [known limitations and planned corrections](TODO.md), particularly for maps, imported types, optional enums, and multiple generated files in one Go package. Validate the types your service uses before relying on these representations.

### Disable envelopes

Set `DisableEnvelope: true` in the generated service options to exchange raw payloads:

```go
pb.RegisterGreeterFrankenPHPServer(server, pb.GreeterFrankenPHPOptions{
    DisableEnvelope: true,
})
```

PHP then receives only the message payload and returns only the response payload. The generated method path and envelope version are not sent, and envelope-based error handling is disabled. Your PHP integration must provide any routing it needs. This setting applies to the whole registered service.

Raw payload mode is incompatible with the Symfony bundle's standard `GrpcBridge` worker. For that integration, leave envelopes enabled and use `GrpcBridge` at the worker boundary. Calling the bundle's dispatcher directly bypasses response wrapping, exception mapping, and service reset.

## Native Go overrides

Each generated options struct contains an optional function for every service method. A non-nil function handles that method directly; methods left nil continue to use PHP.

```go
// This snippet requires the context import.
pb.RegisterGreeterFrankenPHPServer(server, pb.GreeterFrankenPHPOptions{
    SayHello: func(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
        return &pb.HelloReply{Message: "Hello from Go, " + in.GetName()}, nil
    },
})
```

Streaming overrides receive the corresponding generated Go gRPC server stream. Keep overrides in handwritten files.

## Streaming: current behavior and limitations

**The current PHP streaming conventions are experimental and will change.** They do not provide a general interactive streaming API for PHP. Treat the following behavior as the present implementation, not a stable protocol commitment.

| RPC type | Calls to PHP | Request payload | Successful response payload |
| --- | --- | --- | --- |
| Unary | Once per request. | One message array. | One message array. |
| Server streaming | Once per RPC. | One message array. | `['messages' => [/* response message arrays */]]`. |
| Client streaming | Once, after the client finishes sending. | List of all received message arrays. | One message array. |
| Bidirectional streaming | Once per received message. | One message array. | One message array per input. |

With envelopes enabled, these payloads are placed inside the request and response envelopes described above.

Server streaming receives the complete response list from PHP before sending its items. Client streaming buffers all request messages in Go until EOF, without a bridge-specific buffer limit. Bidirectional streaming processes each input and its response in sequence; it does not support independent PHP send and receive operations or multiple responses per input.

With the Symfony bundle's default reset configuration and Symfony's `services_resetter` available, initialized services tagged with `kernel.reset` are reset after each bridge invocation, including failed calls. For the current bidirectional implementation, this means after each message, rather than at the end of the stream. Do not rely on those services retaining state between messages. See the bundle's [service reset configuration](https://github.com/msansen/frankenphp-grpc-bundle#configuration).

Use native Go overrides when a method needs streaming behavior beyond these conventions.

## Generator options

Pass options through `--frankenphp-go-grpc_opt`:

| Option | Default | Purpose |
| --- | --- | --- |
| `file_suffix` | `_frankenphp_grpc.pb.go` | Generated filename suffix; must end in `.go`. |
| `template_dir` | Embedded templates | Directory of custom template overrides. |
| `paths` | `import` | Output placement handled by `protogen`; use `source_relative` to follow source paths. |

For example:

```sh
protoc -I proto \
  --frankenphp-go-grpc_out=generated \
  --frankenphp-go-grpc_opt=paths=source_relative,file_suffix=_php.pb.go \
  proto/greeter.proto
```

### Custom templates

Default Go `text/template` files are embedded in the executable. You can override individual files by placing `.tmpl` files in a custom directory with the same relative paths as the [embedded template tree](internal/generator/templates/). Files you do not override retain their embedded defaults.

For example, `my-templates/methods/unary.tmpl` overrides only the unary method template:

```sh
protoc -I proto \
  --frankenphp-go-grpc_out=generated \
  --frankenphp-go-grpc_opt=paths=source_relative,template_dir=./my-templates \
  proto/greeter.proto
```

Template paths are resolved in the environment where the plugin executes. Template data structures and helper functions are internal interfaces and may change between versions. Compile and test generated code after changing templates or upgrading the plugin.

## Development

```sh
go test ./...
go vet ./...
go build -o bin/protoc-gen-frankenphp-go-grpc ./cmd/protoc-gen-frankenphp-go-grpc
```

With `protoc` and the companion generators installed, generate the fixture with:

```sh
make generate
```

The fixture is in [`testdata/proto/server-test.proto`](testdata/proto/server-test.proto); generated files go into `testdata/generated/`.

Tests currently check generation against fixtures; they do not establish end-to-end compatibility with a running FrankenPHP server. Review and compile generated code, and test it with your chosen runtime versions. Deferred correctness work is tracked in [TODO.md](TODO.md).

## Contributing and support

Bug reports, questions, and pull requests are welcome through [GitHub Issues](https://github.com/msansen/protoc-gen-frankenphp-go-grpc/issues) and [Pull Requests](https://github.com/msansen/protoc-gen-frankenphp-go-grpc/pulls).

For generation bugs, include a minimal `.proto` reproducer, the generation command, generator versions, and the resulting error. For runtime problems, also include FrankenPHP and `frankenphp-grpc` versions and a minimal PHP handler. Add regression coverage when proposing a fix, and run the development checks above.

## License

[MIT](LICENSE).
