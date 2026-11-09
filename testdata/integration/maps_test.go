// This test is copied into the generated bridge package by the integration test.
package bridge

import (
	"math"
	"reflect"
	"testing"

	"github.com/msansen/protoc-gen-frankenphp-go-grpc/testdata/external"
	"github.com/msansen/protoc-gen-frankenphp-go-grpc/testdata/generated"
)

func TestMapKeysRoundTrip(t *testing.T) {
	in := &external.Payload{
		Contacts:   map[string]*generated.Credentials{"main": {Username: "alice"}},
		Statuses:   map[string]generated.Status{"main": generated.Status_STATUS_ACTIVE},
		BoolKeys:   map[bool]string{false: "no", true: "yes"},
		Int32Keys:  map[int32]string{math.MinInt32: "min", math.MaxInt32: "max"},
		Uint32Keys: map[uint32]string{0: "zero", math.MaxUint32: "max"},
		Int64Keys:  map[int64]string{math.MinInt64: "min", math.MaxInt64: "max"},
		Uint64Keys: map[uint64]string{0: "zero", math.MaxUint64: "max"},
	}
	encoded := frankenPHPEncodePayload(in)
	var out external.Payload
	if err := frankenPHPDecodePayload(encoded, &out); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		want, got any
	}{
		{"messages", in.Contacts, out.Contacts},
		{"enums", in.Statuses, out.Statuses},
		{"bool", in.BoolKeys, out.BoolKeys},
		{"int32", in.Int32Keys, out.Int32Keys},
		{"uint32", in.Uint32Keys, out.Uint32Keys},
		{"int64", in.Int64Keys, out.Int64Keys},
		{"uint64", in.Uint64Keys, out.Uint64Keys},
	} {
		if !reflect.DeepEqual(tc.want, tc.got) {
			t.Errorf("%s: want %v, got %v", tc.name, tc.want, tc.got)
		}
	}
}

func TestPHPBooleanMapKeys(t *testing.T) {
	var out external.Payload
	input := map[string]any{"bool_keys": map[string]any{"0": "no", "1": "yes"}}
	if err := frankenPHPDecodePayload(input, &out); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out.BoolKeys, map[bool]string{false: "no", true: "yes"}) {
		t.Fatalf("unexpected bool keys: %v", out.BoolKeys)
	}
}

func TestInvalidMapKeys(t *testing.T) {
	for _, tc := range []struct{ field, key string }{
		{"bool_keys", "invalid"},
		{"bool_keys", "2"},
		{"int32_keys", "2147483648"},
		{"int32_keys", "-2147483649"},
		{"uint32_keys", "4294967296"},
		{"uint32_keys", "-1"},
		{"int64_keys", "9223372036854775808"},
		{"int64_keys", "-9223372036854775809"},
		{"uint64_keys", "18446744073709551616"},
		{"uint64_keys", "-1"},
		{"int64_keys", "1.5"},
		{"uint64_keys", "abc"},
	} {
		t.Run(tc.field+"/"+tc.key, func(t *testing.T) {
			input := map[string]any{tc.field: map[string]any{tc.key: "value"}}
			var out external.Payload
			if err := frankenPHPDecodePayload(input, &out); err == nil {
				t.Fatal("expected invalid key to be rejected")
			}
		})
	}
}
