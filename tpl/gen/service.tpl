package {{ .PackageName }}

import (
	"context"

{{- if ne .PackageName "svc" }}
	"{{ .ProjectName }}/internal/service/svc"
{{- end }}

)

//go:generate mockgen -source=./{{ .FileNameTitleLower }}.go -destination=../../../{{ .AddUPPath }}test/mocks/service/{{ .FilePath }}{{ .FileNameTitleLower }}.go  -package mock_service

var _ {{ .FileName }}Svc = (*{{ .FileNameTitleLower }}Svc)(nil)

type (
	{{ .FileName }}Svc interface {
		Detail(ctx context.Context, id {{ if .PrimaryKeyType }}{{ .PrimaryKeyType }}{{ else }}int64{{ end }}) (*{{ .FileName }}Resp, error)
	}

	{{ .FileName }}Ctx struct {
{{- if eq .PackageName "svc" }}
		*Ctx
{{- else }}
		*svc.Ctx
{{- end }}
	}

	{{ .FileNameTitleLower }}Svc struct {
		ctx *{{ .FileName }}Ctx
	}

	// {{ .FileName }}Resp 业务响应 DTO (与底层的 DB 持久层 DO 实体隔离)
	{{ .FileName }}Resp struct {
		Id {{ if .PrimaryKeyType }}{{ .PrimaryKeyType }}{{ else }}int64{{ end }} `json:"id"`
	}
)

func New{{ .FileName }}Svc(ctx *{{ .FileName }}Ctx) {{ .FileName }}Svc {
	return &{{ .FileNameTitleLower }}Svc{
		ctx: ctx,
	}
}

func ({{ .FileNameFirstChar }} *{{ .FileNameTitleLower }}Svc) Detail(ctx context.Context, id {{ if .PrimaryKeyType }}{{ .PrimaryKeyType }}{{ else }}int64{{ end }}) (*{{ .FileName }}Resp, error) {
	
	// 实现业务详情查询逻辑: 通过 ctx.xxxDb 获取 DO 数据后,转换组装为 DTO 返回
	// TODO: add service logic here and delete this line  
	
	return &{{ .FileName }}Resp{Id: id}, nil
}