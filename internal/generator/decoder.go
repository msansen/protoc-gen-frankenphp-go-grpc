package generator

import (
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type MessageDecoderData struct {
	DecoderName string
	TypeName    string
	HasFields   bool `default:"false"`
	Fields      []DecodeField
}

type DecodeField interface {
	Kind() string
}

type BaseDecodeField struct {
	Key       string
	GoName    string
	Target    string
	ProtoName string
}

type ScalarDecodeField struct {
	BaseDecodeField

	GoType       string
	DecodeHelper string
}

func (ScalarDecodeField) Kind() string {
	return "scalar"
}

type OptionalDecodeField struct {
	BaseDecodeField

	GoType       string
	DecodeHelper string
}

func (OptionalDecodeField) Kind() string {
	return "optional"
}

type MessageDecodeField struct {
	BaseDecodeField

	MessageGoName    string
	QualifiedGoIdent string
	DecoderName      string
}

func (MessageDecodeField) Kind() string {
	return "message"
}

type ListDecodeField struct {
	BaseDecodeField

	HasMessage bool `default:"false"`

	ElementGoType string
	MessageGoName string
	DecodeHelper  string
}

func (ListDecodeField) Kind() string {
	return "list"
}

type MapDecodeField struct {
	BaseDecodeField

	HasMessage bool `default:"false"`

	KeyGoType          string
	ValueGoType        string
	KeyHelper          string
	ValueMessageGoName string
	DecodeHelper       string
	ValueFieldGoType   string
}

func (MapDecodeField) Kind() string {
	return "map"
}

type OneofDecodeField struct {
	BaseDecodeField

	WrapperName   string
	HasMessage    bool
	MessageGoName string
	MessageType   string
	GoType        string
	DecodeHelper  string
}

func (OneofDecodeField) Kind() string {
	return "oneof"
}

// PHP => Go
func newMessageDecoderData(protoFile *protogen.GeneratedFile, message *protogen.Message) (*MessageDecoderData, error) {
	if message == nil {
		return nil, nil
	}

	messageData := &MessageDecoderData{
		DecoderName: "frankenPHPDecode" + message.GoIdent.GoName,
		TypeName:    protoFile.QualifiedGoIdent(message.GoIdent),
	}

	if len(message.Fields) > 0 {
		messageData.HasFields = true
		for _, field := range message.Fields {
			messageData.Fields = append(messageData.Fields, getFieldDataForDecode(protoFile, field))
		}
	}

	return messageData, nil
}

func getFieldDataForDecode(protoFile *protogen.GeneratedFile, field *protogen.Field) DecodeField {
	base := BaseDecodeField{
		Key:       string(field.Desc.Name()),
		GoName:    field.GoName,
		ProtoName: field.GoIdent.GoName,
		Target:    field.GoName,
	}

	if field.Oneof != nil && !field.Oneof.Desc.IsSynthetic() {
		fieldData := &OneofDecodeField{
			BaseDecodeField: base,
			WrapperName:     field.Parent.GoIdent.GoName + "_" + field.GoName,
		}

		fieldData.Target = field.Oneof.GoName

		if field.Message != nil {
			fieldData.HasMessage = true
			fieldData.MessageGoName = field.Message.GoIdent.GoName
			fieldData.MessageType = protoFile.QualifiedGoIdent(field.Message.GoIdent)
		} else {
			fieldData.GoType = goTypeForField(field)
			fieldData.DecodeHelper = decodeHelper(field, "raw")
		}

		return fieldData
	}

	if field.Desc.IsList() {
		fieldData := &ListDecodeField{BaseDecodeField: base}
		fieldData.ElementGoType = goTypeForField(field)
		if field.Message != nil {
			fieldData.HasMessage = true
			fieldData.MessageGoName = field.Message.GoIdent.GoName
		} else {
			fieldData.DecodeHelper = decodeHelper(field, "item")
		}

		return fieldData
	}

	if field.Desc.IsMap() {
		fieldData := &MapDecodeField{BaseDecodeField: base}
		fieldData.KeyGoType = mapKeyType(field.Desc.MapKey().Kind())
		fieldData.ValueGoType = goTypeForField(field)
		fieldData.KeyHelper = "frankenPHPString"
		if field.Desc.MapKey().Kind() != protoreflect.StringKind {
			fieldData.KeyHelper = "frankenPHPInt64"
		}

		// A map is represented by a synthetic MapEntry message.
		// Fields[0] = key, Fields[1] = value.
		valueField := field.Message.Fields[1]
		if valueField.Message != nil {
			fieldData.HasMessage = true
			fieldData.ValueMessageGoName = valueField.Message.GoIdent.GoName
		} else {
			fieldData.DecodeHelper = decodeHelper(field, "item")
			fieldData.ValueFieldGoType = goTypeForField(valueField)
		}

		return fieldData
	}

	if field.Message != nil {
		fieldData := &MessageDecodeField{BaseDecodeField: base}
		fieldData.QualifiedGoIdent = protoFile.QualifiedGoIdent(field.Message.GoIdent)
		fieldData.MessageGoName = field.Message.GoIdent.GoName

		return fieldData
	}

	if isOptionalScalar(field) {
		fieldData := &OptionalDecodeField{BaseDecodeField: base}
		fieldData.GoType = goTypeForField(field)
		fieldData.DecodeHelper = decodeHelper(field, "raw")

		return fieldData
	}

	fieldData := &ScalarDecodeField{BaseDecodeField: base}
	fieldData.GoType = goTypeForField(field)
	fieldData.DecodeHelper = decodeHelper(field, "raw")

	return fieldData
}

func decodeHelper(field *protogen.Field, x string) string {
	kind := field.Desc.Kind()
	switch kind {
	case protoreflect.BoolKind:
		return "frankenPHPBool(" + x + ")"
	case protoreflect.StringKind:
		return "frankenPHPString(" + x + ")"
	case protoreflect.BytesKind:
		return "frankenPHPBytes(" + x + ")"
	case protoreflect.DoubleKind, protoreflect.FloatKind:
		return "frankenPHPFloat64(" + x + ")"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "frankenPHPUint64(" + x + ")"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return "frankenPHPUint64(" + x + ")"
	default:
		return "frankenPHPInt64(" + x + ")"
	}
}

func mapKeyType(kind protoreflect.Kind) string {
	switch kind {
	case protoreflect.StringKind:
		return "string"
	case protoreflect.BoolKind:
		return "bool"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return "int32"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return "int64"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return "uint32"
	default:
		return "uint64"
	}
}
