package generator

import (
	"sort"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type CodecRegistry struct {
	encoders map[protoreflect.FullName]*protogen.Message
	decoders map[protoreflect.FullName]*protogen.Message
}

func NewCodecRegistry() *CodecRegistry {
	return &CodecRegistry{
		encoders: make(map[protoreflect.FullName]*protogen.Message),
		decoders: make(map[protoreflect.FullName]*protogen.Message),
	}
}

func (r *CodecRegistry) RequireEncoder(message *protogen.Message) {
	r.registerMessage(r.encoders, message)
}

func (r *CodecRegistry) RequireDecoder(message *protogen.Message) {
	r.registerMessage(r.decoders, message)
}

func (r *CodecRegistry) SortedEncoders() []*protogen.Message {
	return r.sortedMessages(r.encoders)
}

func (r *CodecRegistry) SortedDecoders() []*protogen.Message {
	return r.sortedMessages(r.decoders)
}

func (r *CodecRegistry) registerMessage(
	registry map[protoreflect.FullName]*protogen.Message,
	message *protogen.Message,
) {
	if message == nil {
		return
	}

	name := message.Desc.FullName()

	if _, exists := registry[name]; exists {
		return
	}

	// Important : enregistrer AVANT de parcourir les enfants.
	registry[name] = message

	for _, field := range message.Fields {
		if field.Desc.IsMap() {
			// A map is represented by a synthetic MapEntry message.
			// Fields[0] = key, Fields[1] = value.
			valueField := field.Message.Fields[1]
			r.registerMessage(registry, valueField.Message)
			continue
		}

		r.registerMessage(registry, field.Message)
	}
}

func (r *CodecRegistry) sortedMessages(registry map[protoreflect.FullName]*protogen.Message) []*protogen.Message {
	names := make([]string, 0, len(registry))

	for name := range registry {
		names = append(names, string(name))
	}

	sort.Strings(names)

	messages := make([]*protogen.Message, 0, len(names))

	for _, name := range names {
		messages = append(
			messages,
			registry[protoreflect.FullName(name)],
		)
	}

	return messages
}
