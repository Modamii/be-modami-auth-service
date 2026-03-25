package response

import (
	"errors"
	"net/http"

	"github.com/cenfit/be-cenfit-auth-service/internal/entity"
	"github.com/gin-gonic/gin"
)

type Response struct {
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{Data: data})
}

func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Response{Data: data})
}

func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func Error(c *gin.Context, err error) {
	var appErr *entity.AppError
	if errors.As(err, &appErr) {
		c.AbortWithStatusJSON(appErr.Code, Response{Error: appErr.Message})
		return
	}
	c.AbortWithStatusJSON(http.StatusInternalServerError, Response{Error: "internal server error"})
}
