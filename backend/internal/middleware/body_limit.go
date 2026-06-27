package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const defaultRequestBodyLimit int64 = 1 << 20

func RequestBodyLimit(limit int64) gin.HandlerFunc {
	if limit <= 0 {
		limit = defaultRequestBodyLimit
	}
	return func(c *gin.Context) {
		if c.Request.ContentLength > limit {
			payload := gin.H{"message": "request entity too large"}
			if requestID := CurrentRequestID(c); requestID != "" {
				payload["requestId"] = requestID
			}
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, payload)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}
