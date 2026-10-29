package generator

import (
	"context"
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
