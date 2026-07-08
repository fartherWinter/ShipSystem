package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"shipsystem/backend/internal/services"
)

const ClaimsKey = "claims"
const TokenCookieName = "shipsystem_token"

const (
	RoleSuperAdmin = "super_admin"
	RoleAdmin      = "admin"
	RoleDispatcher = "dispatcher"
	RoleViewer     = "viewer"
	RoleAnalytics  = "analytics_service"
)

func JWT(auth *services.AuthService, serviceToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenText := tokenFromRequest(c)
		if tokenText == "" {
			AbortWithError(c, http.StatusUnauthorized, "缺少认证令牌")
			return
		}
		if serviceTokenMatches(bearerTokenFromHeader(c), serviceToken) {
			c.Set(ClaimsKey, &services.Claims{Username: "analytics-service", RoleCode: RoleAnalytics})
			c.Next()
			return
		}
		claims, err := auth.ParseToken(tokenText)
		if err != nil {
			AbortWithError(c, http.StatusUnauthorized, "认证令牌无效")
			return
		}
		c.Set(ClaimsKey, claims)
		c.Next()
	}
}

func tokenFromRequest(c *gin.Context) string {
	if token := bearerTokenFromHeader(c); token != "" {
		return token
	}
	token, err := c.Cookie(TokenCookieName)
	if err != nil {
		return ""
	}
	return token
}

func bearerTokenFromHeader(c *gin.Context) string {
	header := c.GetHeader("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimPrefix(header, "Bearer ")
	}
	return ""
}

func serviceTokenMatches(tokenText string, serviceToken string) bool {
	if tokenText == "" || serviceToken == "" || len(tokenText) != len(serviceToken) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(tokenText), []byte(serviceToken)) == 1
}

func RequireRoles(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles)+1)
	allowed[RoleSuperAdmin] = struct{}{}
	for _, role := range roles {
		allowed[role] = struct{}{}
	}
	return func(c *gin.Context) {
		claims := CurrentClaims(c)
		if claims == nil {
			AbortWithError(c, http.StatusUnauthorized, "缺少认证上下文")
			return
		}
		if _, ok := allowed[claims.RoleCode]; !ok {
			AbortWithError(c, http.StatusForbidden, "无权访问该资源")
			return
		}
		c.Next()
	}
}

func AbortWithError(c *gin.Context, status int, message string) {
	payload := gin.H{"message": message}
	if requestID := CurrentRequestID(c); requestID != "" {
		payload["requestId"] = requestID
	}
	c.AbortWithStatusJSON(status, payload)
}

func CurrentClaims(c *gin.Context) *services.Claims {
	value, exists := c.Get(ClaimsKey)
	if !exists {
		return nil
	}
	claims, ok := value.(*services.Claims)
	if !ok {
		return nil
	}
	return claims
}

func CurrentUserID(c *gin.Context) uint {
	claims := CurrentClaims(c)
	if claims == nil {
		return 0
	}
	return claims.UserID
}
