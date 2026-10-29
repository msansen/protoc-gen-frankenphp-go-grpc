package generator

type Config struct {
	FileSuffix  string
	TemplateDir string
}

func DefaultConfig() Config {
	return Config{
		FileSuffix: "_frankenphp_grpc.pb.go",
	}
}
