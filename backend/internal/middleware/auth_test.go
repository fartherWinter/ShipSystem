package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"shipsystem/backend/internal/config"
	"shipsystem/backend/internal/services"
)

func TestRequireRolesAllowsExplicitRole(t *testing.T) {
	recorder := exerciseRequireRoles(t, RoleDispatcher, RequireRoles(RoleAdmin, RoleDispatcher))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestRequireRolesAllowsSuperAdmin(t *testing.T) {
	recorder := exerciseRequireRoles(t, RoleSuperAdmin, RequireRoles(RoleDispatcher))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestRequireRolesWithoutExplicitRolesAllowsOnlySuperAdmin(t *testing.T) {
	superAdminRecorder := exerciseRequireRoles(t, RoleSuperAdmin, RequireRoles())
	if superAdminRecorder.Code != http.StatusOK {
		t.Fatalf("expected super admin 200, got %d body=%s", superAdminRecorder.Code, superAdminRecorder.Body.String())
	}

	adminRecorder := exerciseRequireRoles(t, RoleAdmin, RequireRoles())
	if adminRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected admin 403, got %d body=%s", adminRecorder.Code, adminRecorder.Body.String())
	}
}

func TestRequireRolesRejectsUnauthorizedRole(t *testing.T) {
	recorder := exerciseRequireRoles(t, RoleViewer, RequireRoles(RoleDispatcher))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestRequireRolesRejectsMissingClaims(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.GET("/test", RequireRoles(RoleDispatcher), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(RequestIDHeader, "trace-missing-claims")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	body := decodeJSONBody(t, recorder)
	if body["requestId"] != "trace-missing-claims" {
		t.Fatalf("expected requestId trace-missing-claims, got %#v", body)
	}
}

func TestTokenFromRequestPrefersAuthorizationHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer header-token")
	req.AddCookie(&http.Cookie{Name: TokenCookieName, Value: "cookie-token"})
	ctx.Request = req

	if token := tokenFromRequest(ctx); token != "header-token" {
		t.Fatalf("expected header token, got %q", token)
	}
}

func TestTokenFromRequestFallsBackToCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{Name: TokenCookieName, Value: "cookie-token"})
	ctx.Request = req

	if token := tokenFromRequest(ctx); token != "cookie-token" {
		t.Fatalf("expected cookie token, got %q", token)
	}
}

func TestJWTAcceptsConfiguredServiceToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(JWT(nil, "service-callback-token"))
	router.GET("/test", func(c *gin.Context) {
		claims := CurrentClaims(c)
		if claims == nil {
			t.Fatal("expected service claims")
		}
		if claims.RoleCode != RoleAnalytics || claims.Username != "analytics-service" {
			t.Fatalf("unexpected service claims: %#v", claims)
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer service-callback-token")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestJWTDoesNotAcceptEmptyServiceToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := services.NewAuthService(nil, config.Config{JWTSecret: "test-secret"})
	router := gin.New()
	router.Use(RequestID())
	router.Use(JWT(auth, ""))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(RequestIDHeader, "trace-invalid-token")
	req.Header.Set("Authorization", "Bearer not-a-jwt")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	body := decodeJSONBody(t, recorder)
	if body["requestId"] != "trace-invalid-token" {
		t.Fatalf("expected requestId trace-invalid-token, got %#v", body)
	}
}

func TestRequireRolesAllowsAnalyticsOnlyWhenExplicit(t *testing.T) {
	recorder := exerciseRequireRoles(t, RoleAnalytics, RequireRoles(RoleDispatcher, RoleAnalytics))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected analytics service 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	forbidden := exerciseRequireRoles(t, RoleAnalytics, RequireRoles(RoleDispatcher))
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("expected analytics service 403, got %d body=%s", forbidden.Code, forbidden.Body.String())
	}
}

func TestServiceTokenComparisonRequiresExactValue(t *testing.T) {
	if !serviceTokenMatches("service-token", "service-token") {
		t.Fatal("expected exact token match")
	}
	for _, token := range []string{"", "service-token ", "service-token-extra", "wrong-token"} {
		if serviceTokenMatches(token, "service-token") {
			t.Fatalf("expected token %q to be rejected", token)
		}
	}
	if serviceTokenMatches("service-token", "") {
		t.Fatal("expected empty configured service token to be rejected")
	}
	if serviceTokenMatches(strings.ToUpper("service-token"), "service-token") {
		t.Fatal("expected token comparison to be case-sensitive")
	}
}

func exerciseRequireRoles(t *testing.T, roleCode string, guard gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ClaimsKey, &services.Claims{UserID: 1, Username: "tester", RoleCode: roleCode})
		c.Next()
	})
	router.GET("/test", guard, func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func decodeJSONBody(t *testing.T, recorder *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body: %v body=%s", err, recorder.Body.String())
	}
	return body
}
