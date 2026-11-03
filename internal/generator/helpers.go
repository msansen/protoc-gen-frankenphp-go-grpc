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
