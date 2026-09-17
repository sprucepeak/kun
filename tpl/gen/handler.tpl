package {{ .PackageName }}

import (
	"github.com/gin-gonic/gin"

	"{{ .ProjectName }}/pkg/xerror"
	"{{ .ProjectName }}/pkg/xhttp"
)

type (
	{{ .FileName }}Handler struct {
	}

	// {{ .FileName }}DetailReq 接口请求参数 DTO
	{{ .FileName }}DetailReq struct {
		Id {{ if .PrimaryKeyType }}{{ .PrimaryKeyType }}{{ else }}int64{{ end }} `form:"id" json:"id" binding:"required"`
	}
)

func ({{ .FileNameFirstChar }} *{{ .FileName }}Handler) Detail(ctx *gin.Context) {
	req := &{{ .FileName }}DetailReq{}
	if err := ctx.ShouldBind(req); err != nil {
		xhttp.BusCode(ctx, xerror.ParamError, err)
		return
	}

	// 调用对应 service 获取数据并转换响应
	// TODO: add handler logic here and delete this line 

	xhttp.Data(ctx, "{{ .FileName }} Detail success", req)
}