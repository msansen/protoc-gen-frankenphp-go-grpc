package generator

import (
	"fmt"
	"go/token"
	"path"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
)

type Config struct {
	OutputPackage string
	FileSuffix    string
	TemplateDir   string
}

func DefaultConfig() Config {
	return Config{
		FileSuffix: "_frankenphp_grpc.pb.go",
	}
}

// parseOutputPackage accepts the same import/path;package_name notation as go_package.
func parseOutputPackage(value string) (protogen.GoImportPath, protogen.GoPackageName, error) {
	if value == "" {
		return "", "", nil
	}
	importPath, name, explicit := strings.Cut(value, ";")
	if importPath == "" || path.IsAbs(importPath) || path.Clean(importPath) != importPath || strings.ContainsAny(importPath, " \t\n\r\\:;") || strings.HasPrefix(importPath, ".") {
		return "", "", fmt.Errorf("invalid output_package import path %q", importPath)
	}
	if !explicit {
		name = path.Base(importPath)
	}
	if !token.IsIdentifier(name) || name == "_" {
		return "", "", fmt.Errorf("invalid output_package name %q; use import/path;package_name", name)
	}
	return protogen.GoImportPath(importPath), protogen.GoPackageName(name), nil
}
