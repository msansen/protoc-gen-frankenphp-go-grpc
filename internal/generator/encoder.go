package generator

import (
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type MessageEncoderData struct {
	EncoderName string
	TypeName    string
	HasFields   bool `default:"false"`
	Fields      []EncodeField
}

type EncodeField interface {
	Kind() string
}

type BaseEncodeField struct {
	Key       string
	GoName    string
	Target    string
	ProtoName string
	Expr      string
}

type ScalarEncodeField struct {
	BaseEncodeField
}

func (ScalarEncodeField) Kind() string {
	return "scalar"
}

type OptionalEncodeField struct {
	BaseEncodeField
}

func (OptionalEncodeField) Kind() string {
	return "optional"
}

type MessageEncodeField struct {
	BaseEncodeField

	MessageGoName    string
	QualifiedGoIdent string
	EncoderName      string
}

func (MessageEncodeField) Kind() string {
	return "message"
}

type ListEncodeField struct {
	BaseEncodeField
}

func (ListEncodeField) Kind() string {
	return "list"
}

type MapEncodeField struct {
	BaseEncodeField

	IsStringKey bool `default:"false"`
}

func (MapEncodeField) Kind() string {
	return "map"
}

type OneofEncodeField struct {
	BaseEncodeField

	WrapperName string
}

func (OneofEncodeField) Kind() string {
	return "oneof"
}

// Go => PHP
func newMessageEncoderData(protoFile *protogen.GeneratedFile, message *protogen.Message) (*MessageEncoderData, error) {
	if message == nil {
		return nil, nil
	}

	messageData := &MessageEncoderData{
		EncoderName: "frankenPHPEncode" + message.GoIdent.GoName,
		TypeName:    protoFile.QualifiedGoIdent(message.GoIdent),
	}

	if len(message.Fields) > 0 {
		messageData.HasFields = true
		for _, field := range message.Fields {
			messageData.Fields = append(messageData.Fields, getFieldDataForEncode(field))
		}
	}

	return messageData, nil
}

func getFieldDataForEncode(field *protogen.Field) EncodeField {
	base := BaseEncodeField{
		Key:       string(field.Desc.Name()),
		GoName:    field.GoName,
		ProtoName: field.GoIdent.GoName,
		Target:    field.GoName,
	}
	accessor := fmt.Sprintf("in.%s", field.GoName)

	if field.Oneof != nil && !field.Oneof.Desc.IsSynthetic() {
		fieldData := &OneofEncodeField{BaseEncodeField: base}
		fieldData.Target = field.Oneof.GoName
		fieldData.WrapperName = field.Parent.GoIdent.GoName + "_" + field.GoName
		fieldData.Expr = encodeExpr(field, "in.Get"+field.GoName+"()")

		return fieldData
	}

	if field.Desc.IsList() {
		fieldData := &ListEncodeField{BaseEncodeField: base}
		fieldData.Expr = encodeExpr(field, "item")

		return fieldData
	}

	if field.Desc.IsMap() {
		fieldData := &MapEncodeField{BaseEncodeField: base}
		fieldData.Expr = encodeExpr(field, "item")
		fieldData.IsStringKey = field.Desc.MapKey().Kind() == protoreflect.StringKind

		return fieldData
	}

	if field.Message != nil {
		fieldData := &MessageEncodeField{BaseEncodeField: base}
		fieldData.Expr = encodePresenceExpr(field, accessor)

		return fieldData
	}

	if isOptionalScalar(field) {
		fieldData := &OptionalEncodeField{BaseEncodeField: base}
		fieldData.Expr = encodeExpr(field, accessor)

		return fieldData
	}

	fieldData := &ScalarEncodeField{BaseEncodeField: base}
	fieldData.Expr = encodeExpr(field, accessor)

	return fieldData
}

func encodeExpr(field *protogen.Field, x string) string {
	// A map is represented by a synthetic MapEntry message.
	// Fields[0] = key, Fields[1] = value.
	if field.Message != nil && field.Desc.IsMap() {
		return encodeExpr(field.Message.Fields[1], x)
	}

	if field.Message != nil {
		return "frankenPHPEncode" + field.Message.GoIdent.GoName + "(" + x + ")"
	}

	if field.Enum != nil {
		return "int64(" + x + ")"
	}

	if isOptionalScalar(field) {
		return encodeScalar(field.Desc.Kind(), "*"+x)
	}

	return encodeScalar(field.Desc.Kind(), x)
}

func encodePresenceExpr(field *protogen.Field, x string) string {
	if field.Message != nil {
		return "frankenPHPEncode" + field.Message.GoIdent.GoName + "(" + x + ")"
	}
	return encodeScalar(field.Desc.Kind(), "*"+x)
}

func encodeScalar(kind protoreflect.Kind, x string) string {
	switch kind {
	case protoreflect.FloatKind:
		return "float64(" + x + ")"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return "int64(" + x + ")"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return "int64(" + x + ")"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "strconv.FormatUint(uint64(" + x + "), 10)"
	case protoreflect.BytesKind:
		return "string(" + x + ")"
	default:
		return x
	}
}
