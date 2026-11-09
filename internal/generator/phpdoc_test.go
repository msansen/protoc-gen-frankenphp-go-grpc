package generator

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bufbuild/protocompile"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func phpdocPlugin(t *testing.T) *protogen.Plugin {
	t.Helper()
	compiler := protocompile.Compiler{Resolver: &protocompile.SourceResolver{
		ImportPaths: []string{filepath.Join("..", "..", "testdata", "proto")},
	}}
	files, err := compiler.Compile(context.Background(), "phpdoc-test.proto")
	if err != nil {
		t.Fatal(err)
	}
	plugin, err := (protogen.Options{}).New(&pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{"phpdoc-test.proto"},
		ProtoFile:      []*descriptorpb.FileDescriptorProto{protodesc.ToFileDescriptorProto(files[0])},
	})
	if err != nil {
		t.Fatal(err)
	}
	return plugin
}

func TestHandlerPHPDocTypes(t *testing.T) {
	plugin := phpdocPlugin(t)
	method := plugin.Files[0].Services[0].Methods[0]
	got := strings.Join(handlerPHPDoc(method), "\n")
	for _, want := range []string{
		"name: string,", "nickname?: string,", "data: string,",
		"large_id: numeric-string,", "fixed_id: numeric-string,", "ids?: list<numeric-string>,",
		"blobs?: array<array-key, string>,", "children?: array<array-key, array{",
		"nodes?: list<array{", "child?: array{", "value: string,", "}|null>",
		"status: int,", "ratio: float,", "amount: float,", "enabled: bool,", "count: int,", "offset: int,", "code: int,",
		"token?: string,", "selected?: array{", "tree?: array{",
		"next?: array<string, mixed>,", "children?: list<array<string, mixed>|null>,",
		"indexed?: array<array-key, array<string, mixed>|null>,",
		"} $request\n */",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("PHPDoc missing %q\n%s", want, got)
		}
	}
	// A shared Child is expanded on each sibling, rather than being mistaken for
	// a cycle because it was visited earlier in a different field.
	if count := strings.Count(got, "value: string,"); count != 5 {
		t.Errorf("expected 5 expanded message shapes, got %d\n%s", count, got)
	}
	if len(got) > 5000 {
		t.Fatalf("recursive shape was not bounded: %d characters", len(got))
	}
}

func TestGeneratedMethodsHavePHPDoc(t *testing.T) {
	plugin := phpdocPlugin(t)
	if err := Generate(plugin, DefaultConfig(), "test"); err != nil {
		t.Fatal(err)
	}
	response := plugin.Response()
	if response.GetError() != "" {
		t.Fatal(response.GetError())
	}
	file, err := parser.ParseFile(token.NewFileSet(), "generated.go", response.File[0].GetContent(), parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	found := make(map[string]bool)
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil {
			continue
		}
		name := fn.Name.Name
		found[name] = true
		if fn.Doc == nil {
			t.Errorf("method %s has no attached comment", name)
			continue
		}
		doc := fn.Doc.Text()
		want := "@param array{"
		if name == "ClientStream" {
			want = "@param list<array{"
		}
		if name == "EmptyRequest" {
			want = "@param array{} $request"
		}
		if !strings.Contains(doc, want) || !strings.Contains(doc, "$request") {
			t.Errorf("%s PHPDoc missing %q\n%s", name, want, doc)
		}
		if strings.Contains(doc, "__frankenphp_grpc") || strings.Contains(doc, "version:") || strings.Contains(doc, "payload:") {
			t.Errorf("%s PHPDoc contains the envelope: %s", name, doc)
		}
	}
	for _, name := range []string{"Unary", "ServerStream", "ClientStream", "BidiStream", "EmptyRequest"} {
		if !found[name] {
			t.Errorf("missing generated method %s", name)
		}
	}
}
