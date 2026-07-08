package middleware

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"shipsystem/backend/internal/services"
)

func TestRequestIDGeneratesMissingID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.GET("/test", func(c *gin.Context) {
		if CurrentRequestID(c) == "" {
			t.Fatal("expected request id in context")
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", recorder.Code)
	}
	if recorder.Header().Get(RequestIDHeader) == "" {
		t.Fatal("expected response request id header")
	}
}

func TestRequestIDReusesValidHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.GET("/test", func(c *gin.Context) {
		if CurrentRequestID(c) != "client-request-123" {
			t.Fatalf("expected client request id in context, got %q", CurrentRequestID(c))
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(RequestIDHeader, "client-request-123")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Header().Get(RequestIDHeader) != "client-request-123" {
		t.Fatalf("expected response to reuse request id, got %q", recorder.Header().Get(RequestIDHeader))
	}
}

func TestRequestIDReplacesInvalidHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.GET("/test", func(c *gin.Context) {
		if CurrentRequestID(c) == "bad id" {
			t.Fatal("expected invalid request id to be replaced")
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(RequestIDHeader, "bad id")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Header().Get(RequestIDHeader) == "" || recorder.Header().Get(RequestIDHeader) == "bad id" {
		t.Fatalf("expected generated request id, got %q", recorder.Header().Get(RequestIDHeader))
	}
}

func TestRequestLoggerIncludesRequestIDAndRouteFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	logger := log.New(&output, "", 0)

	router := gin.New()
	router.Use(RequestID())
	router.Use(RequestLogger(logger))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusAccepted)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(RequestIDHeader, "trace-123")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	logLine := output.String()
	for _, expected := range []string{
		"request_id=trace-123",
		"user_id=anonymous",
		"role=anonymous",
		"method=GET",
		"path=/test",
		"status=202",
		"latency_ms=",
		"client_ip=",
	} {
		if !strings.Contains(logLine, expected) {
			t.Fatalf("expected log line to contain %q, got %q", expected, logLine)
		}
	}
}

func TestRequestLoggerIncludesAuthenticatedActorFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	logger := log.New(&output, "", 0)

	router := gin.New()
	router.Use(RequestID())
	router.Use(func(c *gin.Context) {
		c.Set(ClaimsKey, &services.Claims{UserID: 42, RoleCode: RoleDispatcher})
		c.Next()
	})
	router.Use(RequestLogger(logger))
	router.GET("/secure", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	req.Header.Set(RequestIDHeader, "trace-authenticated")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	logLine := output.String()
	for _, expected := range []string{
		"request_id=trace-authenticated",
		"user_id=42",
		"role=dispatcher",
		"method=GET",
		"path=/secure",
		"status=200",
	} {
		if !strings.Contains(logLine, expected) {
			t.Fatalf("expected authenticated log line to contain %q, got %q", expected, logLine)
		}
	}
}

func TestRecoveryReturnsJSONWithRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	logger := log.New(&output, "", 0)

	router := gin.New()
	router.Use(RequestID())
	router.Use(Recovery(logger))
	router.GET("/panic", func(c *gin.Context) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	req.Header.Set(RequestIDHeader, "trace-panic")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected JSON content type, got %q", contentType)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body: %v body=%s", err, recorder.Body.String())
	}
	if body["message"] != "服务器内部错误" || body["requestId"] != "trace-panic" {
		t.Fatalf("unexpected recovery body: %#v", body)
	}

	logLine := output.String()
	for _, expected := range []string{
		"request_id=trace-panic",
		"user_id=anonymous",
		"role=anonymous",
		"panic=boom",
		"method=GET",
		"path=/panic",
	} {
		if !strings.Contains(logLine, expected) {
			t.Fatalf("expected recovery log to contain %q, got %q", expected, logLine)
		}
	}
}
