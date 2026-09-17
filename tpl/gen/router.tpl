
package {{ .PackageName }}

import (
    "{{ .ProjectName }}/internal/handler"
    "{{ .ProjectName }}/pkg/token"

    "github.com/gin-gonic/gin"
)


func {{ .FileName }}(e *gin.Engine, jwt *token.Jwt, ctx *handler.Ctx) {
	// TODO: add routes here and delete this line
}

