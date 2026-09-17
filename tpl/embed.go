package tpl

import "embed"

//go:embed gen/*.tpl
var GenTplFS embed.FS

//go:embed basic.zip advanced.zip
var NewTplZipFS embed.FS
