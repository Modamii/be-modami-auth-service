package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	logging "gitlab.com/lifegoeson-libs/pkg-logging"
)

func ZapLogger(logger logging.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		attrs := []logging.Attr{
			logging.Int("status", c.Writer.Status()),
			logging.String("method", c.Request.Method),
			logging.String("path", path),
			logging.String("query", query),
			logging.String("ip", c.ClientIP()),
			logging.String("latency", time.Since(start).String()),
			logging.Int("size", c.Writer.Size()),
		}

		if requestID, exists := c.Get("request_id"); exists {
			attrs = append(attrs, logging.String("request_id", requestID.(string)))
		}

		status := c.Writer.Status()
		switch {
		case status >= 500:
			logger.Error("request", nil, attrs...)
		case status >= 400:
			logger.Warn("request", attrs...)
		default:
			logger.Info("request", attrs...)
		}
	}
}
