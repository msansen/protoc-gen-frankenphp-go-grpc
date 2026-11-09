package generator

import (
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Keep inline shapes finite for recursive schemas and reasonably sized for deeply
// nested inputs. Recursive PHPStan aliases cannot describe arbitrary trees.
const maxPHPDocDepth = 8

func handlerPHPDoc(method *protogen.Method) []string {
	shape := phpstanMessageType(method.Input, make(map[protoreflect.FullName]bool), 0)
	// Client streaming buffers all inputs. Bidi streaming invokes PHP once per
	// message, so only client-only streaming has a list as its root payload.
	if method.Desc.IsStreamingClient() && !method.Desc.IsStreamingServer() {
		shape = "list<" + shape + ">"
	}
	lines := []string{"/**"}
	for i, line := range strings.Split(shape, "\n") {
		if i == 0 {
			line = "@param " + line
		}
		lines = append(lines, " * "+line)
	}
	lines[len(lines)-1] += " $request"
	return append(lines, " */")
}

func phpstanMessageType(message *protogen.Message, active map[protoreflect.FullName]bool, depth int) string {
	name := message.Desc.FullName()
	if active[name] || depth >= maxPHPDocDepth {
		return "array<string, mixed>"
	}
	if len(message.Fields) == 0 {
		return "array{}"
	}
	active[name] = true
	defer delete(active, name)

	indent := strings.Repeat("    ", depth)
	var shape strings.Builder
	shape.WriteString("array{\n")
	for _, field := range message.Fields {
		key := string(field.Desc.Name())
		// Lists/maps and singular messages are omitted when nil. Presence-bearing
		// scalars and oneof alternatives are likewise only emitted when selected.
		if field.Desc.IsList() || field.Desc.IsMap() || field.Message != nil || field.Desc.HasPresence() {
			key += "?"
		}
		shape.WriteString(indent + "    " + key + ": " + phpstanFieldType(field, active, depth+1) + ",\n")
	}
	shape.WriteString(indent + "}")
	return shape.String()
}

func phpstanFieldType(field *protogen.Field, active map[protoreflect.FullName]bool, depth int) string {
	if field.Desc.IsMap() {
		value := field.Message.Fields[1]
		valueType := phpstanValueType(value, active, depth)
		if value.Message != nil {
			valueType += "|null"
		}
		// PHP can normalize numeric string keys to integers, even for string-keyed
		// protobuf maps. array-key covers both representations at the PHP boundary.
		return "array<array-key, " + valueType + ">"
	}
	valueType := phpstanValueType(field, active, depth)
	if field.Desc.IsList() {
		if field.Message != nil {
			valueType += "|null"
		}
		return "list<" + valueType + ">"
	}
	return valueType
}

func phpstanValueType(field *protogen.Field, active map[protoreflect.FullName]bool, depth int) string {
	if field.Message != nil {
		return phpstanMessageType(field.Message, active, depth)
	}
	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		return "bool"
	case protoreflect.StringKind, protoreflect.BytesKind:
		return "string"
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return "float"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "numeric-string"
	default:
		return "int"
	}
}
