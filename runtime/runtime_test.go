package runtime

import (
	"testing"

	"github.com/dunglas/frankenphp"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestUnwrapResponse(t *testing.T) {
	tests := []struct {
		name     string
		response any
		code     codes.Code
	}{
		{"success", map[string]any{"__frankenphp_grpc": map[string]any{"version": int64(1), "ok": true, "payload": "response"}}, codes.OK},
		{"PHP error", map[string]any{"__frankenphp_grpc": map[string]any{"version": int64(1), "ok": false, "error": map[string]any{"code": int64(3), "message": "invalid request"}}}, codes.InvalidArgument},
		{"missing envelope", map[string]any{}, codes.Internal},
		{"unsupported version", map[string]any{"__frankenphp_grpc": map[string]any{"version": int64(2), "ok": true, "payload": "response"}}, codes.Internal},
		{"missing payload", map[string]any{"__frankenphp_grpc": map[string]any{"version": int64(1), "ok": true}}, codes.Internal},
		{"invalid error code", map[string]any{"__frankenphp_grpc": map[string]any{"version": int64(1), "ok": false, "error": map[string]any{"code": int64(0), "message": "bad"}}}, codes.Internal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := UnwrapResponse(tt.response, 1)
			if status.Code(err) != tt.code {
				t.Fatalf("got %v, want code %v", err, tt.code)
			}
			if tt.code == codes.OK && payload != "response" {
				t.Fatalf("unexpected payload %v", payload)
			}
			if tt.code == codes.InvalidArgument && status.Convert(err).Message() != "invalid request" {
				t.Fatalf("lost PHP error: %v", err)
			}
		})
	}
}

func TestUint64(t *testing.T) {
	const max = ^uint64(0)
	got, err := Uint64("18446744073709551615")
	if err != nil || got != max {
		t.Fatalf("got %d, %v", got, err)
	}
	for _, value := range []any{int64(-1), "18446744073709551616", true} {
		if _, err := Uint64(value); err == nil {
			t.Errorf("expected error for %v", value)
		}
	}
}

func TestResponseSlice(t *testing.T) {
	items, err := ResponseSlice(map[string]any{"messages": []any{"first", "second"}}, "messages")
	if err != nil || len(items) != 2 {
		t.Fatalf("got %v, %v", items, err)
	}
	if _, err := ResponseSlice(map[string]any{}, "messages"); err == nil {
		t.Fatal("expected error for missing messages")
	}
	if _, err := Map([]any{}); err == nil {
		t.Fatal("expected error for a list")
	}
}

func TestFrankenPHPArrays(t *testing.T) {
	payload := frankenphp.AssociativeArray[any]{Map: map[string]any{"messages": []any{"response"}}}
	response := frankenphp.AssociativeArray[any]{Map: map[string]any{
		"__frankenphp_grpc": frankenphp.AssociativeArray[any]{Map: map[string]any{
			"version": int64(1), "ok": true, "payload": payload,
		}},
	}}
	decoded, err := UnwrapResponse(response, 1)
	if err != nil {
		t.Fatal(err)
	}
	items, err := ResponseSlice(decoded, "messages")
	if err != nil || len(items) != 1 || items[0] != "response" {
		t.Fatalf("got %v, %v", items, err)
	}
	if _, err := Map(42); err == nil {
		t.Fatal("expected unsupported value to be rejected")
	}
	failure := frankenphp.AssociativeArray[any]{Map: map[string]any{
		"__frankenphp_grpc": frankenphp.AssociativeArray[any]{Map: map[string]any{
			"version": int64(1), "ok": false,
			"error": frankenphp.AssociativeArray[any]{Map: map[string]any{"code": int64(7), "message": "denied"}},
		}},
	}}
	_, err = UnwrapResponse(failure, 1)
	if status.Code(err) != codes.PermissionDenied || status.Convert(err).Message() != "denied" {
		t.Fatalf("unexpected error %v", err)
	}
}
