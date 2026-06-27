package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"shipsystem/backend/internal/observability"
)

const RequestIDHeader = observability.RequestIDHeader
const RequestIDKey = "request_id"

const maxRequestIDLength = 128

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader(RequestIDHeader))
		if !validRequestID(requestID) {
			requestID = newRequestID()
		}
		c.Set(RequestIDKey, requestID)
		c.Request.Header.Set(RequestIDHeader, requestID)
		c.Request = c.Request.WithContext(observability.ContextWithRequestID(c.Request.Context(), requestID))
		c.Header(RequestIDHeader, requestID)
		c.Next()
	}
}

func CurrentRequestID(c *gin.Context) string {
	value, exists := c.Get(RequestIDKey)
	if !exists {
		return ""
	}
	requestID, ok := value.(string)
	if !ok {
		return ""
	}
	return requestID
}

func RequestLogger(logger *log.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		c.Next()
		if logger == nil {
			return
		}
		userID, roleCode := currentRequestActor(c)
		logger.Printf(
			"request_id=%s user_id=%s role=%s method=%s path=%s status=%d latency_ms=%d client_ip=%s",
			CurrentRequestID(c),
			userID,
			roleCode,
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			time.Since(startedAt).Milliseconds(),
			c.ClientIP(),
		)
	}
}

func Recovery(logger *log.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		requestID := CurrentRequestID(c)
		userID, roleCode := currentRequestActor(c)
		if logger != nil {
			logger.Printf(
				"request_id=%s user_id=%s role=%s panic=%v method=%s path=%s client_ip=%s",
				requestID,
				userID,
				roleCode,
				recovered,
				c.Request.Method,
				c.Request.URL.Path,
				c.ClientIP(),
			)
		}
		payload := gin.H{"message": "服务器内部错误"}
		if requestID != "" {
			payload["requestId"] = requestID
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, payload)
	})
}

func validRequestID(value string) bool {
	if value == "" || len(value) > maxRequestIDLength {
		return false
	}
	for _, r := range value {
		if r < 33 || r > 126 {
			return false
		}
	}
	return true
}

func newRequestID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return hex.EncodeToString(bytes[:])
	}
	return hex.EncodeToString([]byte(time.Now().UTC().Format("20060102150405.000000000")))
}

func currentRequestActor(c *gin.Context) (string, string) {
	claims := CurrentClaims(c)
	if claims == nil {
		return "anonymous", "anonymous"
	}
	userID := "anonymous"
	if claims.UserID > 0 {
		userID = strings.TrimSpace(strconv.FormatUint(uint64(claims.UserID), 10))
	}
	roleCode := strings.TrimSpace(claims.RoleCode)
	if roleCode == "" {
		roleCode = "unknown"
	}
	return userID, roleCode
}
