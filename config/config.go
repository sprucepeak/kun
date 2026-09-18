package config

const (
	Version         = "1.3.4"
	Slogan          = "\n     _    _\n    | |  / )\n    | | / /_   _ ____\n    | |< <| | | |  _ \\\n    | | \\ \\ |_| | | | |\n    |_|  \\_)____|_| |_|\n\n A CLI tool for building golang application. \n"
	WireUrl         = "github.com/google/wire/cmd/wire@latest"
	MockgenUrl      = "go.uber.org/mock/mockgen@latest"
	GolangciLintUrl = "github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
	GovulnUrl       = "golang.org/x/vuln/cmd/govulncheck@latest"
	NilawayUrl      = "go.uber.org/nilaway/cmd/nilaway@latest"
	SwagUrl         = "github.com/swaggo/swag/cmd/swag@latest"
	KunUrl          = "github.com/sprucepeak/kun@latest"
	RunExcludeDir   = ".git,.idea,tmp,vendor"
	RunIncludeExt   = "go,html,yaml,yml,toml,ini,json,xml,tpl,tmpl"
	Short           = Slogan + " Kun " + Version + " - © 2026 SprucePeak\n Released under the MIT License.\n \n"
)
