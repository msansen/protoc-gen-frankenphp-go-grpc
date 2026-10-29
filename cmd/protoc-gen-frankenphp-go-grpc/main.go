package main

import (
	"flag"

	"github.com/msansen/protoc-gen-frankenphp-go-grpc/internal/generator"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

var version = "dev"

func main() {
	var flags flag.FlagSet
	var paths string

	cfg := generator.DefaultConfig()
	flags.StringVar(&cfg.FileSuffix, "file_suffix", cfg.FileSuffix, "suffix of generated Go files")
	flags.StringVar(
		&cfg.TemplateDir,
		"template_dir",
		cfg.TemplateDir,
		"directory containing template overrides (defaults to templates embedded in the binary)",
	)
	flags.StringVar(
		&paths,
		"paths",
		"",
		"accepted for protoc convention compatibility; output placement is handled by protogen",
	)

	opts := protogen.Options{ParamFunc: flags.Set}
	opts.Run(
		func(plugin *protogen.Plugin) error {
			plugin.SupportedFeatures =
				uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)

			return generator.Generate(plugin, cfg, version)
		},
	)
}
