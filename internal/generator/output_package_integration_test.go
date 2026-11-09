package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/compiler/protogen"
)

// Compile against actual protoc-gen-go and protoc-gen-go-grpc output. Only the
// native PHP boundary is stubbed, so this test also runs without PHP headers.
func TestOutputPackageCompilesAndDecodesMaps(t *testing.T) {
	for _, tool := range []string{"protoc", "protoc-gen-go", "protoc-gen-go-grpc"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is required for the compilation test", tool)
		}
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	write := func(name string, content []byte) {
		t.Helper()
		destination := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	read := func(name string) []byte {
		t.Helper()
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		return content
	}
	module := "github.com/msansen/protoc-gen-frankenphp-go-grpc"
	mod := string(read("go.mod")) + `
require github.com/dunglas/frankenphp-grpc v0.0.0
replace github.com/dunglas/frankenphp => ./stub/frankenphp
replace github.com/dunglas/frankenphp-grpc => ./stub/grpc
`
	write("go.mod", []byte(mod))
	write("go.sum", read("go.sum"))
	for _, name := range []string{"helpers.go", "bridge.go"} {
		write("runtime/"+name, read("runtime/"+name))
	}
	write("stub/frankenphp/go.mod", []byte("module github.com/dunglas/frankenphp\ngo 1.27\n"))
	write("stub/frankenphp/stub.go", []byte("package frankenphp\ntype AssociativeArray[T any] struct { Map map[string]T }\n"))
	write("stub/grpc/go.mod", []byte("module github.com/dunglas/frankenphp-grpc\ngo 1.27\n"))
	write("stub/grpc/stub.go", []byte(`package grpc
import "context"
func HandleRequest(context.Context, any) (any, error) { return nil, nil }
`))
	run := func(name string, args ...string) {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOWORK=off", "CGO_ENABLED=0")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, output)
		}
	}
	run("protoc", "-I"+filepath.Join(root, "testdata", "proto"),
		"--go_out=.", "--go_opt=module="+module,
		"--go-grpc_out=.", "--go-grpc_opt=module="+module,
		"server-test.proto", "external-package-test.proto")
	req := externalPackageRequest(t)
	req.FileToGenerate = []string{"server-test.proto", "external-package-test.proto"}
	parameter := "module=" + module
	req.Parameter = &parameter
	plugin, err := (protogen.Options{}).New(req)
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.OutputPackage = module + "/testdata/bridge"
	if err := Generate(plugin, cfg, "test"); err != nil {
		t.Fatal(err)
	}
	response := plugin.Response()
	if response.GetError() != "" {
		t.Fatal(response.GetError())
	}
	for _, file := range response.File {
		write(file.GetName(), []byte(file.GetContent()))
	}
	write("testdata/bridge/maps_test.go", read("testdata/integration/maps_test.go"))
	run("go", "test", "-mod=mod", "./testdata/bridge")
}
