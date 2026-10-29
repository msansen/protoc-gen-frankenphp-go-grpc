package generator

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
)

var (
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
	HelpersData     helpersData
}

type serviceData struct {
	OptionsName      string
	ServerName       string
	ServiceName      string
	ConstructorName  string
	RegisterName     string
	ServiceRegistrar string
	Methods          []methodData
}

type methodData struct {
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

type helpersData struct {
	AssociativeArray string
	Errorf           string
	ParseUint        string
}

func Generate(plugin *protogen.Plugin, cfg Config, version string) error {
	if cfg.FileSuffix == "" || !strings.HasSuffix(cfg.FileSuffix, ".go") {
		return fmt.Errorf("file_suffix must end in .go")
	}

	for _, file := range plugin.Files {
		if !file.Generate || len(file.Services) == 0 {
			continue
		}
		if err := generateFile(plugin, file, cfg, version); err != nil {
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
) error {
	protoFile := plugin.NewGeneratedFile(file.GeneratedFilenamePrefix+cfg.FileSuffix, file.GoImportPath)
	templates, err := loadTemplates(cfg.TemplateDir, protoFile)
	if err != nil {
		return fmt.Errorf("load templates: %w", err)
	}

	var body Builder
	codecRegistry := NewCodecRegistry()

	fileData := &fileData{
		Version: version,
		Source:  file.Desc.Path(),
		Package: string(file.GoPackageName),
		Body:    body.String(),
	}

	for _, service := range file.Services {
		for _, method := range service.Methods {
			codecRegistry.RequireEncoder(method.Input)
			codecRegistry.RequireDecoder(method.Output)
		}
	}

	for _, service := range file.Services {
		serviceData, err := newServiceData(protoFile, service, cfg)

		if err != nil {
			return fmt.Errorf("render service %s: %w", service.Desc.FullName(), err)
		}
		fileData.Services = append(fileData.Services, *serviceData)
	}

	// Generate concrete codecs only for messages reachable from RPC inputs/outputs.
	for _, message := range codecRegistry.SortedEncoders() {
		messageFullName := message.Desc.FullName()

		encoder, err := newMessageEncoderData(protoFile, message)
		if err != nil {
			return fmt.Errorf("render encoder  %s: %w", messageFullName, err)
		}

		fileData.MessageEncoders = append(fileData.MessageEncoders, *encoder)
	}

	for _, message := range codecRegistry.SortedDecoders() {
		messageFullName := message.Desc.FullName()

		decoder, err := newMessageDecoderData(protoFile, message)
		if err != nil {
			return fmt.Errorf("render decoder  %s: %w", messageFullName, err)
		}

		fileData.MessageDecoders = append(fileData.MessageDecoders, *decoder)
	}

	fileData.HelpersData = helpersData{
		AssociativeArray: protoFile.QualifiedGoIdent(frankenphpPackage.Ident("AssociativeArray[any]")),
		Errorf:           protoFile.QualifiedGoIdent(fmtPackage.Ident("Errorf")),
		ParseUint:        protoFile.QualifiedGoIdent(strconvPackage.Ident("ParseUint")),
	}

	content, err := executeTemplate(templates, "file.tmpl", fileData)

	if err != nil {
		return err
	}

	protoFile.P(content)

	return nil
}

func newServiceData(protoFile *protogen.GeneratedFile, service *protogen.Service, cfg Config) (*serviceData, error) {
	serverName := service.GoName + "FrankenPHPServer"
	data := &serviceData{
		OptionsName:      service.GoName + "FrankenPHPOptions",
		ServerName:       serverName,
		ServiceName:      service.GoName,
		ConstructorName:  "New" + service.GoName + "FrankenPHPServer",
		RegisterName:     "Register" + service.GoName + "FrankenPHPServer",
		ServiceRegistrar: protoFile.QualifiedGoIdent(grpcPackage.Ident("ServiceRegistrar")),
	}

	for _, method := range service.Methods {
		methodData, err := newMethodData(protoFile, service, method, serverName, cfg)
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
	cfg Config,
) (*methodData, error) {
	fullMethod := fmt.Sprintf("/%s/%s", service.Desc.FullName(), method.Desc.Name())
	data := methodData{
		ServerName:        serverName,
		MethodName:        method.GoName,
		Context:           protoFile.QualifiedGoIdent(contextPackage.Ident("Context")),
		Input:             protoFile.QualifiedGoIdent(method.Input.GoIdent),
		Output:            protoFile.QualifiedGoIdent(method.Output.GoIdent),
		Stream:            service.GoName + "_" + method.GoName + "Server",
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
