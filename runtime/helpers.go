// Package runtime provides the Go/PHP conversions used by gRPC handlers generated with protoc-gen-frankenphp-go-grpc.
package runtime

import (
	"fmt"
	"strconv"

	"github.com/dunglas/frankenphp"
)

// Map accepts a Go map or a FrankenPHP associative array.
func Map(v any) (map[string]any, error) {
	if a, ok := v.(frankenphp.AssociativeArray[any]); ok {
		return a.Map, nil
	}
	if m, ok := v.(map[string]any); ok {
		return m, nil
	}
	return nil, fmt.Errorf("expected PHP associative array, got %T", v)
}

// List accepts a PHP list represented as a Go slice.
func List(v any) ([]any, error) {
	if x, ok := v.([]any); ok {
		return x, nil
	}
	return nil, fmt.Errorf("expected PHP list, got %T", v)
}

// ResponseSlice reads a named list from a response payload.
func ResponseSlice(v any, key string) ([]any, error) {
	m, e := Map(v)
	if e != nil {
		return nil, e
	}
	return List(m[key])
}

// String accepts a PHP string.
func String(v any) (string, error) {
	if x, ok := v.(string); ok {
		return x, nil
	}
	return "", fmt.Errorf("expected string, got %T", v)
}

// Bytes converts a binary PHP string to a byte slice.
func Bytes(v any) ([]byte, error) {
	x, e := String(v)
	return []byte(x), e
}

// Bool accepts a PHP boolean.
func Bool(v any) (bool, error) {
	if x, ok := v.(bool); ok {
		return x, nil
	}
	return false, fmt.Errorf("expected bool, got %T", v)
}

// Int64 converts supported PHP-facing numeric values to an integer.
func Int64(v any) (int64, error) {
	switch x := v.(type) {
	case int64:
		return x, nil
	case int:
		return int64(x), nil
	case int32:
		return int64(x), nil
	case uint32:
		return int64(x), nil
	case float64:
		return int64(x), nil
	}
	return 0, fmt.Errorf("expected integer, got %T", v)
}

// Uint64 accepts an unsigned integer or its decimal string representation.
func Uint64(v any) (uint64, error) {
	switch x := v.(type) {
	case string:
		return strconv.ParseUint(x, 10, 64)
	case int64:
		if x < 0 {
			return 0, fmt.Errorf("negative unsigned integer")
		}
		return uint64(x), nil
	case uint64:
		return x, nil
	case uint32:
		return uint64(x), nil
	}
	return 0, fmt.Errorf("expected unsigned integer/string, got %T", v)
}

// Float64 converts supported PHP-facing numeric values to a float.
func Float64(v any) (float64, error) {
	switch x := v.(type) {
	case float64:
		return x, nil
	case float32:
		return float64(x), nil
	case int64:
		return float64(x), nil
	}
	return 0, fmt.Errorf("expected float, got %T", v)
}
