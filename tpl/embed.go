package tpl

import "embed"

//go:embed gen/*.tpl
var GenTplFS embed.FS

//go:embed api.zip service.zip
var NewTplZipFS embed.FS
