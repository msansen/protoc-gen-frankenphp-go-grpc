package generator

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Builder struct {
	strings.Builder
}

func (b *Builder) Writef(format string, args ...any) error {
	_, err := fmt.Fprintf(b, format, args...)

	return err
}

func isOptionalScalar(field *protogen.Field) bool {
	return field.Desc.HasPresence() &&
		!field.Desc.IsList() &&
		!field.Desc.IsMap() &&
		field.Message == nil &&
		(field.Oneof == nil || field.Oneof.Desc.IsSynthetic())
}

func goTypeForField(field *protogen.Field) string {
	if field.Enum != nil {
		return field.Enum.GoIdent.GoName
	}

	// A map is represented by a synthetic MapEntry message.
	// Fields[0] = key, Fields[1] = value.
	if field.Desc.IsMap() {
		return goTypeForField(field.Message.Fields[1])
	}

	if field.Message != nil {
		return "*" + field.Message.GoIdent.GoName
	}

	if field.Enum != nil {
		return field.Enum.GoIdent.GoName
	}

	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		return "bool"
	case protoreflect.StringKind:
		return "string"
	case protoreflect.BytesKind:
		return "[]byte"
	case protoreflect.DoubleKind:
		return "float64"
	case protoreflect.FloatKind:
		return "float32"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return "int64"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "uint64"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return "uint32"
	default:
		return "int32"
	}
}

//func frankenPHPUnwrapResponse(
//	response any,
//	expectedVersion int64,
//) (any, error) {
//	invalid := func() (any, error) {
//		return nil, status.Error(
//			codes.Internal,
//			"Invalid FrankenPHP gRPC response envelope",
//		)
//	}
//
//	// Read the root PHP associative array.
//	root, ok := frankenPHPEnvelopeMap(response)
//	if !ok {
//		return invalid()
//	}
//
//	// Retrieve the protocol envelope.
//	envelope, ok := frankenPHPEnvelopeMap(root["__frankenphp_grpc"])
//	if !ok {
//		return invalid()
//	}
//
//	// Validate the protocol version.
//	version, ok := frankenPHPEnvelopeInt(envelope["version"])
//	if !ok || version != expectedVersion {
//		return nil, status.Error(
//			codes.Internal,
//			"Unsupported FrankenPHP gRPC envelope version",
//		)
//	}
//
//	// Validate the response status.
//	success, ok := envelope["ok"].(bool)
//	if !ok {
//		return invalid()
//	}
//
//	if success {
//		payload, exists := envelope["payload"]
//		if !exists {
//			return invalid()
//		}
//
//		return payload, nil
//	}
//
//	// Decode the error returned by PHP.
//	grpcError, ok := frankenPHPEnvelopeMap(envelope["error"])
//	if !ok {
//		return invalid()
//	}
//
//	code, ok := frankenPHPEnvelopeInt(grpcError["code"])
//	if !ok || code < 1 || code > 16 {
//		return invalid()
//	}
//
//	message, ok := grpcError["message"].(string)
//	if !ok {
//		return invalid()
//	}
//
//	return nil, status.Error(
//		codes.Code(code),
//		message,
//	)
//}
//
//// PHP associative arrays are not necessarily Go maps.
//// FrankenPHP can return AssociativeArray[any].
//func frankenPHPEnvelopeMap(value any) (map[string]any, bool) {
//	switch v := value.(type) {
//	case map[string]any:
//		return v, true
//
//	case frankenphp.AssociativeArray[any]:
//		return v.Map, true
//
//	default:
//		return nil, false
//	}
//}
//
//// PHP integers are normally represented as int64
//// when converted through FrankenPHP.
//func frankenPHPEnvelopeInt(value any) (int64, bool) {
//	switch v := value.(type) {
//	case int:
//		return int64(v), true
//
//	case int32:
//		return int64(v), true
//
//	case int64:
//		return v, true
//
//	default:
//		return 0, false
//	}
//}
