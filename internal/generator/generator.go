package generator

import (
	"fmt"
	"path"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
)

var (
	runtimePackage    = protogen.GoImportPath("github.com/msansen/protoc-gen-frankenphp-go-grpc/runtime")
	contextPackage    = protogen.GoImportPath("context")
	frankenphpPackage = protogen.GoImportPath("github.com/dunglas/frankenphp")
	phpGrpcPackage    = protogen.GoImportPath("github.com/dunglas/frankenphp-grpc")
	grpcPackage       = protogen.GoImportPath("google.golang.org/grpc")
	grpcCodesPackage  = protogen.GoImportPath("google.golang.org/grpc/codes")
	grpcStatusPackage = protogen.GoImportPath("google.golang.org/grpc/status")
	fmtPackage        = protogen.GoImportPath("fmt")
	strconvPackage    = protogen.GoImportPath("strconv")
	ioPackage         = protogen.GoImportPath("io")
)

type fileData struct {
	Version         string
	Source          string
	Package         string
	Body            string
	Services        []serviceData
	MessageEncoders []MessageEncoderData
	MessageDecoders []MessageDecoderData
}

type serviceData struct {
	OptionsName         string
	ServerName          string
	ServiceName         string
	ConstructorName     string
	RegisterName        string
	ServiceRegistrar    string
	UnimplementedServer string
	RegisterService     string
	Methods             []methodData
}

type methodData struct {
	PHPDoc                []string
	ServerName            string
	MethodName            string
	Context               string
	ContextExpr           string
	Input                 string
	Output                string
	Stream                string
	EOF                   string
	RequestEncoder        string
	ResponseDecoder       string
	StreamItemDecoder     string
	RequestData           requestData
	IsUnary               bool `default:"false"`
	IsClientStream        bool `default:"false"`
	IsServerStream        bool `default:"false"`
	IsBidirectionalStream bool `default:"false"`
}
type requestData struct {
	MethodName        string
	HandlerMethodName string
	NeedEnvelope      bool `default:"true"`
}

func Generate(plugin *protogen.Plugin, cfg Config, version string) error {
	if cfg.FileSuffix == "" || !strings.HasSuffix(cfg.FileSuffix, ".go") {
		return fmt.Errorf("file_suffix must end in .go")
	}

	outputPath, outputName, err := parseOutputPackage(cfg.OutputPackage)
	if err != nil {
		return err
	}
	// Codec declarations are package-scoped, even when services span proto files.
	codecsByPackage := make(map[protogen.GoImportPath]*CodecRegistry)
	for _, file := range plugin.Files {
		if !file.Generate || len(file.Services) == 0 {
			continue
		}

		// Handles different output package.
		packagePath := file.GoImportPath
		if outputPath != "" {
			packagePath = outputPath
			if packagePath == file.GoImportPath && outputName != file.GoPackageName {
				return fmt.Errorf("output_package name %q conflicts with existing package name %q for %s", outputName, file.GoPackageName, packagePath)
			}
		}

		registry := codecsByPackage[packagePath]
		if registry == nil {
			registry = NewCodecRegistry()
			codecsByPackage[packagePath] = registry
		}

		if err := generateFile(plugin, file, cfg, version, registry, packagePath, outputName); err != nil {
			return err
		}
	}

	return nil
}

func generateFile(
	plugin *protogen.Plugin,
	file *protogen.File,
	cfg Config,
	version string,
	emittedCodecs *CodecRegistry,
	packagePath protogen.GoImportPath,
	outputName protogen.GoPackageName,
) error {
	prefix := file.GeneratedFilenamePrefix

	// Handles different output package.
	packageName := file.GoPackageName
	if outputName != "" {
		packageName = outputName
		sourceRelative := false
		for _, parameter := range strings.Split(plugin.Request.GetParameter(), ",") {
			if name, value, _ := strings.Cut(parameter, "="); name == "paths" {
				sourceRelative = value == "source_relative"
			}
		}
		if !sourceRelative {
			prefix = path.Join(string(packagePath), path.Base(prefix))
		}
	}

	protoFile := plugin.NewGeneratedFile(prefix+cfg.FileSuffix, packagePath)
	templates, err := loadTemplates(cfg.TemplateDir, protoFile)
	if err != nil {
		return fmt.Errorf("load templates: %w", err)
	}

	var body Builder
	codecRegistry := NewCodecRegistry()

	fileData := &fileData{
		Version: version,
		Source:  file.Desc.Path(),
		Package: string(packageName),
		Body:    body.String(),
	}

	for _, service := range file.Services {
		for _, method := range service.Methods {
			codecRegistry.RequireEncoder(method.Input)
			codecRegistry.RequireDecoder(method.Output)
		}
	}

	for _, service := range file.Services {
		serviceData, err := newServiceData(protoFile, service, file.GoImportPath, cfg)

		if err != nil {
			return fmt.Errorf("render service %s: %w", service.Desc.FullName(), err)
		}
		fileData.Services = append(fileData.Services, *serviceData)
	}

	// Generate concrete codecs only for messages reachable from RPC inputs/outputs.
	for _, message := range codecRegistry.SortedEncoders() {
		messageFullName := message.Desc.FullName()
		if _, exists := emittedCodecs.encoders[messageFullName]; exists {
			continue
		}
		emittedCodecs.encoders[messageFullName] = message

		encoder, err := newMessageEncoderData(protoFile, message)
		if err != nil {
			return fmt.Errorf("render encoder  %s: %w", messageFullName, err)
		}

		fileData.MessageEncoders = append(fileData.MessageEncoders, *encoder)
	}

	for _, message := range codecRegistry.SortedDecoders() {
		messageFullName := message.Desc.FullName()
		if _, exists := emittedCodecs.decoders[messageFullName]; exists {
			continue
		}
		emittedCodecs.decoders[messageFullName] = message

		decoder, err := newMessageDecoderData(protoFile, message)
		if err != nil {
			return fmt.Errorf("render decoder  %s: %w", messageFullName, err)
		}

		fileData.MessageDecoders = append(fileData.MessageDecoders, *decoder)
	}

	content, err := executeTemplate(templates, "file.tmpl", fileData)

	if err != nil {
		return err
	}

	protoFile.P(content)

	return nil
}

func newServiceData(protoFile *protogen.GeneratedFile, service *protogen.Service, serviceImportPath protogen.GoImportPath, cfg Config) (*serviceData, error) {
	serverName := service.GoName + "FrankenPHPServer"
	data := &serviceData{
		OptionsName:         service.GoName + "FrankenPHPOptions",
		ServerName:          serverName,
		ServiceName:         service.GoName,
		ConstructorName:     "New" + service.GoName + "FrankenPHPServer",
		RegisterName:        "Register" + service.GoName + "FrankenPHPServer",
		ServiceRegistrar:    protoFile.QualifiedGoIdent(grpcPackage.Ident("ServiceRegistrar")),
		UnimplementedServer: protoFile.QualifiedGoIdent(serviceImportPath.Ident("Unimplemented" + service.GoName + "Server")),
		RegisterService:     protoFile.QualifiedGoIdent(serviceImportPath.Ident("Register" + service.GoName + "Server")),
	}

	for _, method := range service.Methods {
		methodData, err := newMethodData(protoFile, service, method, serverName, serviceImportPath, cfg)
		if err != nil {
			return nil, err
		}

		data.Methods = append(data.Methods, *methodData)
	}

	return data, nil
}

func newMethodData(
	protoFile *protogen.GeneratedFile,
	service *protogen.Service,
	method *protogen.Method,
	serverName string,
	serviceImportPath protogen.GoImportPath,
	cfg Config,
) (*methodData, error) {
	fullMethod := fmt.Sprintf("/%s/%s", service.Desc.FullName(), method.Desc.Name())
	data := methodData{
		PHPDoc:            handlerPHPDoc(method),
		ServerName:        serverName,
		MethodName:        method.GoName,
		Context:           protoFile.QualifiedGoIdent(contextPackage.Ident("Context")),
		Input:             protoFile.QualifiedGoIdent(method.Input.GoIdent),
		Output:            protoFile.QualifiedGoIdent(method.Output.GoIdent),
		Stream:            protoFile.QualifiedGoIdent(serviceImportPath.Ident(service.GoName + "_" + method.GoName + "Server")),
		RequestEncoder:    "frankenPHPEncode" + method.Input.GoIdent.GoName,
		ResponseDecoder:   "frankenPHPDecode" + method.Output.GoIdent.GoName,
		StreamItemDecoder: "frankenPHPDecode" + method.Output.GoIdent.GoName,
		RequestData:       newRequestData(protoFile, fullMethod, cfg),
		ContextExpr:       "stream.Context()",
	}

	// +-------------------+-------------------+---------------+
	// | IsStreamingClient | IsStreamingServer | Type          |
	// +-------------------+-------------------+---------------+
	// | false             | false             | Unary         |
	// | false             | true              | Server Stream |
	// | true              | false             | Client Stream |
	// | true              | true              | Bidi Stream   |
	// +-------------------+-------------------+---------------+

	if !method.Desc.IsStreamingClient() && !method.Desc.IsStreamingServer() {
		data.ContextExpr = "ctx"
	}

	// data.EOF only needed for Client/Bidi Streams, don't import io otherwise
	if method.Desc.IsStreamingClient() {
		data.EOF = protoFile.QualifiedGoIdent(ioPackage.Ident("EOF"))
	}

	switch {
	case !method.Desc.IsStreamingClient() && !method.Desc.IsStreamingServer():
		data.IsUnary = true
	case !method.Desc.IsStreamingClient() && method.Desc.IsStreamingServer():
		data.IsServerStream = true
	case method.Desc.IsStreamingClient() && !method.Desc.IsStreamingServer():
		data.IsClientStream = true
	case method.Desc.IsStreamingClient() && method.Desc.IsStreamingServer():
		data.IsBidirectionalStream = true
	}

	return &data, nil
}

func newRequestData(protoFile *protogen.GeneratedFile, method string, cfg Config) requestData {
	return requestData{
		MethodName:        method,
		HandlerMethodName: protoFile.QualifiedGoIdent(phpGrpcPackage.Ident("HandleRequest")),
	}
}
