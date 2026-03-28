package response

import (
	"github.com/gin-gonic/gin"
	"gitlab.com/lifegoeson-libs/pkg-gokit/apperror"
	pkgresponse "gitlab.com/lifegoeson-libs/pkg-gokit/response"
)

func OK(c *gin.Context, data any) {
	pkgresponse.OK(c.Writer, data)
}

func Created(c *gin.Context, data any) {
	pkgresponse.Created(c.Writer, data)
}

func NoContent(c *gin.Context) {
	pkgresponse.NoContent(c.Writer)
}

func OKWithPagination(c *gin.Context, data any, p pkgresponse.Pagination) {
	pkgresponse.OKWithPagination(c.Writer, data, p)
}

func Error(c *gin.Context, err error) {
	if ae := apperror.AsAppError(err); ae != nil {
		pkgresponse.Err(c.Writer, ae)
		c.Abort()
		return
	}
	pkgresponse.InternalError(c.Writer, "internal server error")
	c.Abort()
}

func BadRequest(c *gin.Context, msg string) {
	pkgresponse.BadRequest(c.Writer, msg)
	c.Abort()
}

func Unauthorized(c *gin.Context, msg string) {
	pkgresponse.Unauthorized(c.Writer, msg)
	c.Abort()
}

func Forbidden(c *gin.Context, msg string) {
	pkgresponse.Forbidden(c.Writer, msg)
	c.Abort()
}

func NotFound(c *gin.Context, msg string) {
	pkgresponse.NotFound(c.Writer, msg)
	c.Abort()
}

func InternalError(c *gin.Context, msg string) {
	pkgresponse.InternalError(c.Writer, msg)
	c.Abort()
}
