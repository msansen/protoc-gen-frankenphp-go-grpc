package generator

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/bufbuild/protocompile"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestGenerateAllRPCShapes(t *testing.T) {
	content := generateFixture(t)

	for _, want := range []string{
		"type TestServiceFrankenPHPOptions struct",
		"func (s *TestServiceFrankenPHPServer) Unary(",
		"func (s *TestServiceFrankenPHPServer) ServerStream(",
		"func (s *TestServiceFrankenPHPServer) ClientStream(",
		"func (s *TestServiceFrankenPHPServer) BidiStream(",
		"\"method\":  \"/test.v1.TestService/Unary\"",
		"func frankenPHPEncodeStreamRequest(in *StreamRequest) map[string]any",
		"out[\"sequence\"] = int64(in.Sequence)",
		"func frankenPHPDecodeStreamResponse(value any, out *StreamResponse) error",

		`out["password"] = *in.Password`,
		`out["priority"] = int64(*in.Priority)`,
		`out["enabled"] = *in.Enabled`,

		`in.Authentication.(*UnaryRequest_Token)`,
		`in.Authentication.(*UnaryRequest_UserId)`,
		`in.Authentication.(*UnaryRequest_Credentials)`,
		`out["token"] = in.GetToken()`,
		`out["user_id"] = int64(in.GetUserId())`,
		`out["credentials"] = frankenPHPEncodeCredentials(in.GetCredentials())`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("generated output does not contain %q\n%s", want, content)
		}
	}

	if strings.Contains(content, "mapstructure") {
		t.Fatal("generated output must not use mapstructure")
	}
}

func TestMapKeyEncoding(t *testing.T) {
	content := generateFixture(t)

	cases := []struct {
		name      string
		field     string
		keyExpr   string
		wrongExpr string
	}{
		{
			name:      "string key",
			field:     "Metadata",
			keyExpr:   "key",
			wrongExpr: "fmt.Sprint(key)",
		},
		{
			name:      "int32 key",
			field:     "Prices",
			keyExpr:   "fmt.Sprint(key)",
			wrongExpr: "key",
		},
	}

	for _, tc := range cases {
		t.Run(
			tc.name, func(t *testing.T) {
				prefix := `for key, item := range in\.` + tc.field + `\s*\{\s*`
				correct := regexp.MustCompile(prefix + `v\[` + regexp.QuoteMeta(tc.keyExpr) + `\] = item`)
				if !correct.MatchString(content) {
					t.Fatalf("expected key expression %q when encoding %s\n%s", tc.keyExpr, tc.field, content)
				}

				incorrect := regexp.MustCompile(prefix + `v\[` + regexp.QuoteMeta(tc.wrongExpr) + `\] = item`)
				if incorrect.MatchString(content) {
					t.Fatalf("unexpected key expression %q when encoding %s", tc.wrongExpr, tc.field)
				}
			},
		)
	}

	// A numeric map key needs fmt.Sprint in the generated code.
	if !strings.Contains(content, `"fmt"`) {
		t.Fatal("generated output must import fmt for non-string map keys")
	}
}

func TestRejectsInvalidSuffix(t *testing.T) {
	plugin, err := (protogen.Options{}).New(fixtureRequest(t))
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.FileSuffix = "bad"
	if err := Generate(plugin, cfg, "test"); err == nil {
		t.Fatal("expected an error")
	}
}

func generateFixture(t *testing.T) string {
	t.Helper()

	req := fixtureRequest(t)
	plugin, err := (protogen.Options{}).New(req)
	if err != nil {
		t.Fatal(err)
	}
	if err := Generate(plugin, DefaultConfig(), "test"); err != nil {
		t.Fatal(err)
	}

	resp := plugin.Response()
	if len(resp.File) != 1 {
		t.Fatalf("expected 1 generated file, got %d", len(resp.File))
	}
	return resp.File[0].GetContent()
}

func fixtureRequest(t *testing.T) *pluginpb.CodeGeneratorRequest {
	t.Helper()

	const filename = "server-test.proto"

	compiler := protocompile.Compiler{
		Resolver: &protocompile.SourceResolver{
			ImportPaths: []string{
				filepath.Join("..", "..", "testdata", "proto"),
			},
		},
	}

	files, err := compiler.Compile(context.Background(), filename)
	if err != nil {
		t.Fatalf("compile fixture %s: %v", filename, err)
	}

	request := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{filename},
	}

	for _, file := range files {
		request.ProtoFile = append(
			request.ProtoFile,
			protodesc.ToFileDescriptorProto(file),
		)
	}

	return request
}

func TestCustomTemplateDirectory(t *testing.T) {
	dir := t.TempDir()

	// Preserve the complete template tree so nested template names still resolve.
	err := fs.WalkDir(
		defaultTemplates, "templates", func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if name == "templates" {
				return nil
			}

			relative := strings.TrimPrefix(name, "templates/")
			destination := filepath.Join(dir, filepath.FromSlash(relative))
			if entry.IsDir() {
				return os.MkdirAll(destination, 0o755)
			}

			content, err := fs.ReadFile(defaultTemplates, name)
			if err != nil {
				return err
			}
			if relative == "file.tmpl" {
				content = []byte("// CUSTOM TEMPLATE\n" + string(content))
			}
			return os.WriteFile(destination, content, 0o600)
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	plugin, err := (protogen.Options{}).New(fixtureRequest(t))
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.TemplateDir = dir
	if err := Generate(plugin, cfg, "test"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plugin.Response().File[0].GetContent(), "// CUSTOM TEMPLATE") {
		t.Fatal("custom template directory was not used")
	}
}

func TestInvalidTemplateDirectory(t *testing.T) {
	plugin, err := (protogen.Options{}).New(fixtureRequest(t))
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.TemplateDir = filepath.Join(t.TempDir(), "missing")
	if err := Generate(plugin, cfg, "test"); err == nil {
		t.Fatal("expected an error for missing template directory")
	}
}

func TestMultipleFilesUseSharedRuntime(t *testing.T) {
	compiler := protocompile.Compiler{Resolver: &protocompile.SourceResolver{
		ImportPaths: []string{filepath.Join("..", "..", "testdata", "proto")},
	}}
	names := []string{"server-test.proto", "second-service-test.proto"}
	files, err := compiler.Compile(context.Background(), names...)
	if err != nil {
		t.Fatal(err)
	}
	req := &pluginpb.CodeGeneratorRequest{FileToGenerate: names}
	for _, file := range files {
		req.ProtoFile = append(req.ProtoFile, protodesc.ToFileDescriptorProto(file))
	}
	plugin, err := (protogen.Options{}).New(req)
	if err != nil {
		t.Fatal(err)
	}
	if err := Generate(plugin, DefaultConfig(), "test"); err != nil {
		t.Fatal(err)
	}
	response := plugin.Response()
	if response.GetError() != "" {
		t.Fatal(response.GetError())
	}
	if len(response.File) != 2 {
		t.Fatalf("expected 2 files, got %d", len(response.File))
	}
	codecDeclarations := make(map[string]string)
	codecCalls := make(map[string]bool)
	for _, file := range response.File {
		content := file.GetContent()
		if !strings.Contains(content, `"github.com/msansen/protoc-gen-frankenphp-go-grpc/runtime"`) {
			t.Fatalf("%s does not import the shared runtime", file.GetName())
		}
		for _, want := range []string{"runtime.UnwrapResponse(response, 1)"} {
			if !strings.Contains(content, want) {
				t.Errorf("%s does not call %s", file.GetName(), want)
			}
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file.GetName(), content, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if ok {
				if name, ok := call.Fun.(*ast.Ident); ok &&
					(strings.HasPrefix(name.Name, "frankenPHPEncode") || strings.HasPrefix(name.Name, "frankenPHPDecode")) {
					codecCalls[name.Name] = true
				}
			}
			return true
		})
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil {
				continue
			}
			if strings.HasPrefix(fn.Name.Name, "frankenPHPEncode") || strings.HasPrefix(fn.Name.Name, "frankenPHPDecode") {
				if previous, exists := codecDeclarations[fn.Name.Name]; exists {
					t.Errorf("codec %s declared in both %s and %s", fn.Name.Name, previous, file.GetName())
				}
				codecDeclarations[fn.Name.Name] = file.GetName()
			}
			if strings.HasPrefix(fn.Name.Name, "frankenPHP") &&
				!strings.HasPrefix(fn.Name.Name, "frankenPHPEncode") &&
				!strings.HasPrefix(fn.Name.Name, "frankenPHPDecode") {
				t.Errorf("%s still declares runtime helper %s", file.GetName(), fn.Name.Name)
			}
		}
	}
	for name := range codecCalls {
		if _, exists := codecDeclarations[name]; !exists {
			t.Errorf("codec call %s has no declaration", name)
		}
	}
	for _, name := range []string{"frankenPHPEncodeUnaryRequest", "frankenPHPDecodeUnaryResponse", "frankenPHPEncodeCredentials"} {
		if _, exists := codecDeclarations[name]; !exists {
			t.Errorf("missing shared codec %s", name)
		}
	}
}

func TestOutputPackage(t *testing.T) {
	for _, tc := range []struct{ name, parameter, output, filename, packageName string }{
		{"import paths", "", "example.com/bridge", "example.com/bridge/external-package-test_frankenphp_grpc.pb.go", "bridge"},
		{"source relative", "paths=source_relative", "example.com/bridge", "external-package-test_frankenphp_grpc.pb.go", "bridge"},
		{"module", "module=example.com", "example.com/bridge", "bridge/external-package-test_frankenphp_grpc.pb.go", "bridge"},
		{"explicit name", "", "example.com/bridge;adapter", "example.com/bridge/external-package-test_frankenphp_grpc.pb.go", "adapter"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := externalPackageRequest(t)
			req.Parameter = &tc.parameter
			plugin, err := (protogen.Options{}).New(req)
			if err != nil {
				t.Fatal(err)
			}
			cfg := DefaultConfig()
			cfg.OutputPackage = tc.output
			if err := Generate(plugin, cfg, "test"); err != nil {
				t.Fatal(err)
			}
			resp := plugin.Response()
			if resp.GetError() != "" {
				t.Fatal(resp.GetError())
			}
			if len(resp.File) != 1 {
				t.Fatalf("expected 1 file, got %d", len(resp.File))
			}
			file := resp.File[0]
			if file.GetName() != tc.filename {
				t.Fatalf("filename = %q, want %q", file.GetName(), tc.filename)
			}
			content := file.GetContent()
			for _, want := range []string{
				"package " + tc.packageName,
				`"github.com/msansen/protoc-gen-frankenphp-go-grpc/testdata/external"`,
				`"github.com/msansen/protoc-gen-frankenphp-go-grpc/testdata/generated"`,
				"external.UnimplementedExternalServiceServer",
				"external.RegisterExternalServiceServer(registrar",
				"stream external.ExternalService_ServerStreamServer",
				"stream external.ExternalService_ClientStreamServer",
				"stream external.ExternalService_BidiStreamServer",
				"in *external.Payload",
				"out *external.Payload",
				"in.Choice.(*external.Payload_Identity)",
				"&external.Payload_State{",
				"&generated.Credentials{}",
				"make([]*generated.Credentials",
				"make(map[string]*generated.Credentials",
				"make(map[string]generated.Status",
				"make([]generated.Status",
				"generated.Status(v)",
				"strconv.ParseBool(key)",
				"strconv.ParseInt(key, 10, 32)",
				"strconv.ParseInt(key, 10, 64)",
				"strconv.ParseUint(key, 10, 32)",
				"strconv.ParseUint(key, 10, 64)",
			} {
				if !strings.Contains(content, want) {
					t.Errorf("output missing %q\n%s", want, content)
				}
			}
		})
	}
}

func TestInvalidOutputPackage(t *testing.T) {
	for _, value := range []string{";bridge", "example.com/bridge;", "example.com/bridge;for", "example.com/bridge;_", "example.com/bridge;bad-name", "/absolute", "../relative", "example.com//bridge", "example.com/bridge/", "example.com/bad name"} {
		t.Run(value, func(t *testing.T) {
			plugin, err := (protogen.Options{}).New(fixtureRequest(t))
			if err != nil {
				t.Fatal(err)
			}
			cfg := DefaultConfig()
			cfg.OutputPackage = value
			if err := Generate(plugin, cfg, "test"); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func externalPackageRequest(t *testing.T) *pluginpb.CodeGeneratorRequest {
	t.Helper()
	compiler := protocompile.Compiler{Resolver: &protocompile.SourceResolver{
		ImportPaths: []string{filepath.Join("..", "..", "testdata", "proto")},
	}}
	files, err := compiler.Compile(context.Background(), "server-test.proto", "external-package-test.proto")
	if err != nil {
		t.Fatal(err)
	}
	req := &pluginpb.CodeGeneratorRequest{FileToGenerate: []string{"external-package-test.proto"}}
	for _, file := range files {
		req.ProtoFile = append(req.ProtoFile, protodesc.ToFileDescriptorProto(file))
	}
	return req
}

func TestOutputPackageRejectsConflictingName(t *testing.T) {
	plugin, err := (protogen.Options{}).New(fixtureRequest(t))
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.OutputPackage = string(plugin.Files[0].GoImportPath) + ";bridge"
	if err := Generate(plugin, cfg, "test"); err == nil {
		t.Fatal("expected an error for conflicting names at the same import path")
	}
}
