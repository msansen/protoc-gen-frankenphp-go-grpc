package generator

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
	"text/template"

	"google.golang.org/protobuf/compiler/protogen"
)

// defaultTemplates are compiled into the plugin binary so protoc-gen-frankenphp-go-grpc
// has no runtime dependency on template files.
//
//go:embed templates
var defaultTemplates embed.FS

func loadTemplates(templateDir string, protoFile *protogen.GeneratedFile) (*template.Template, error) {
	embedded, err := fs.Sub(defaultTemplates, "templates")
	if err != nil {
		return nil, fmt.Errorf("access embedded templates: %w", err)
	}

	files, err := loadFS(embedded)
	if err != nil {
		return nil, fmt.Errorf("list embedded templates: %w", err)
	}

	sources := make(map[string]fs.FS, len(files))

	for _, name := range files {
		sources[name] = embedded
	}

	if templateDir != "" {
		overrides := os.DirFS(templateDir)

		files, err := loadFS(overrides)
		if err != nil {
			return nil, fmt.Errorf("list custom templates: %w", err)
		}

		for _, name := range files {
			sources[name] = overrides
		}
	}

	tmpl := template.New("frankenphp-go-grpc").Funcs(templateFunctions(protoFile))

	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		content, err := fs.ReadFile(sources[name], name)
		if err != nil {
			return nil, fmt.Errorf("read template %q: %w", name, err)
		}

		if _, err := tmpl.New(name).Parse(string(content)); err != nil {
			return nil, fmt.Errorf("parse template %q: %w", name, err)
		}
	}

	return tmpl, nil
}

func templateFunctions(protoFile *protogen.GeneratedFile) template.FuncMap {
	qualifiedGoIdent := func(path protogen.GoImportPath) func(string) string {
		return func(name string) string {
			return protoFile.QualifiedGoIdent(path.Ident(name))
		}
	}
	return template.FuncMap{
		"envelopeFormatVersion": func() int {
			return 1
		},
		"context":        qualifiedGoIdent(contextPackage),
		"fmt":            qualifiedGoIdent(fmtPackage),
		"strconv":        qualifiedGoIdent(strconvPackage),
		"io":             qualifiedGoIdent(ioPackage),
		"frankenPHP":     qualifiedGoIdent(frankenphpPackage),
		"frankenPHPGrpc": qualifiedGoIdent(phpGrpcPackage),
		"grpc":           qualifiedGoIdent(grpcPackage),
		"grpcCodes":      qualifiedGoIdent(grpcCodesPackage),
		"grpcStatus":     qualifiedGoIdent(grpcStatusPackage),
	}
}

func loadFS(source fs.FS) ([]string, error) {
	var files []string

	err := fs.WalkDir(
		source, ".", func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if entry.IsDir() || !strings.HasSuffix(name, ".tmpl") {
				return nil
			}

			files = append(files, name)
			return nil
		},
	)

	if err != nil {
		return nil, err
	}

	return files, nil
}

func executeTemplate(template *template.Template, name string, data any) (string, error) {
	var buffer bytes.Buffer
	if err := template.ExecuteTemplate(&buffer, name, data); err != nil {
		return "", fmt.Errorf("execute template %s: %w", name, err)
	}

	return buffer.String(), nil
}
