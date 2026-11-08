package runtime

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnwrapResponse validates a versioned PHP response envelope and returns its
// payload or gRPC status error.
func UnwrapResponse(response any, expectedVersion int64) (any, error) {
	invalid := func() (any, error) {
		return nil, status.Error(
			codes.Internal,
			"Invalid FrankenPHP gRPC response envelope",
		)
	}

	// Read the root PHP associative array
	root, ok := envelopeMap(response)
	if !ok {
		return invalid()
	}

	// Retrieve the protocol envelope
	envelope, ok := envelopeMap(root["__frankenphp_grpc"])
	if !ok {
		return invalid()
	}

	// Validate the protocol version
	version, ok := envelopeInt(envelope["version"])
	if !ok || version != expectedVersion {
		return nil, status.Error(
			codes.Internal,
			"Unsupported FrankenPHP gRPC envelope version",
		)
	}

	// Validate the response status
	success, ok := envelope["ok"].(bool)
	if !ok {
		return invalid()
	}

	if success {
		payload, exists := envelope["payload"]
		if !exists {
			return invalid()
		}

		return payload, nil
	}

	// Decode the error returned by PHP
	grpcError, ok := envelopeMap(envelope["error"])
	if !ok {
		return invalid()
	}

	code, ok := envelopeInt(grpcError["code"])
	if !ok || code < 1 || code > 16 {
		return invalid()
	}

	message, ok := grpcError["message"].(string)
	if !ok {
		return invalid()
	}

	return nil, status.Error(
		codes.Code(code),
		message,
	)
}

func envelopeMap(value any) (map[string]any, bool) {
	m, err := Map(value)
	return m, err == nil
}

// PHP integers are normally represented as int64
// when converted through FrankenPHP
func envelopeInt(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true

	case int32:
		return int64(v), true

	case int64:
		return v, true

	default:
		return 0, false
	}
}
