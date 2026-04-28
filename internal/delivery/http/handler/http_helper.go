package handler

import (
	"github.com/gin-gonic/gin"
	"gitlab.com/lifegoeson-libs/pkg-gokit/apperror"
	pkgresponse "gitlab.com/lifegoeson-libs/pkg-gokit/response"
)

func respondOK(c *gin.Context, data any) {
	pkgresponse.OK(c.Writer, data)
}

func respondCreated(c *gin.Context, data any) {
	pkgresponse.Created(c.Writer, data)
}

func respondNoContent(c *gin.Context) {
	pkgresponse.NoContent(c.Writer)
}

func respondError(c *gin.Context, err error) {
	if ae := apperror.AsAppError(err); ae != nil {
		pkgresponse.Err(c.Writer, ae)
		c.Abort()
		return
	}
	pkgresponse.InternalError(c.Writer, "lỗi máy chủ nội bộ")
	c.Abort()
}
