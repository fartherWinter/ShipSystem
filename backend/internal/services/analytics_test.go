package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"shipsystem/backend/internal/config"
	"shipsystem/backend/internal/observability"
)

func TestAnalyticsServiceSendsAdminToken(t *testing.T) {
	var gotToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("X-Analytics-Token")
		if r.URL.Path != "/simulate/status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"running": true})
	}))
	defer server.Close()

	service := NewAnalyticsService(config.Config{
		AnalyticsBaseURL:    server.URL,
		AnalyticsAdminToken: "backend-only-token",
	})

	data, err := service.Status(context.Background())
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if gotToken != "backend-only-token" {
		t.Fatalf("expected analytics token header, got %q", gotToken)
	}
	if data["running"] != true {
		t.Fatalf("unexpected response: %#v", data)
	}
}

func TestAnalyticsServiceForwardsRequestID(t *testing.T) {
	var gotRequestID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequestID = r.Header.Get(observability.RequestIDHeader)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"running": true})
	}))
	defer server.Close()

	service := NewAnalyticsService(config.Config{AnalyticsBaseURL: server.URL})
	ctx := observability.ContextWithRequestID(context.Background(), "trace-analytics")

	if _, err := service.Status(ctx); err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if gotRequestID != "trace-analytics" {
		t.Fatalf("expected forwarded request id, got %q", gotRequestID)
	}
}

func TestAnalyticsServiceUsesConfiguredHTTPTimeout(t *testing.T) {
	service := NewAnalyticsService(config.Config{
		AnalyticsBaseURL:     "http://analytics:8090",
		AnalyticsHTTPTimeout: 7 * time.Second,
	})

	if service.client.Timeout != 7*time.Second {
		t.Fatalf("expected configured analytics timeout 7s, got %s", service.client.Timeout)
	}
}

func TestAnalyticsServiceStartBattleRequiresSessionID(t *testing.T) {
	service := NewAnalyticsService(config.Config{AnalyticsBaseURL: "http://example.invalid"})

	_, err := service.StartBattleSimulation(context.Background(), BattleSimulationStartRequest{})
	if !IsValidationError(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestAnalyticsServiceRejectsCallsWhenBaseURLIsNotConfigured(t *testing.T) {
	service := NewAnalyticsService(config.Config{})

	_, err := service.Status(context.Background())
	if !IsValidationError(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if err.Error() != "analytics service is not configured" {
		t.Fatalf("expected analytics configuration error, got %q", err.Error())
	}
}

func TestAnalyticsServiceStartBattleNormalizesAndDefaultsPayload(t *testing.T) {
	var payload BattleSimulationStartRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/simulate/battle/start" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"running": true})
	}))
	defer server.Close()

	service := NewAnalyticsService(config.Config{AnalyticsBaseURL: server.URL})
	_, err := service.StartBattleSimulation(context.Background(), BattleSimulationStartRequest{
		SessionID:       " battle-1 ",
		OriginLongitude: 121.49,
		OriginLatitude:  31.23,
	})
	if err != nil {
		t.Fatalf("StartBattleSimulation returned error: %v", err)
	}
	if payload.SessionID != "battle-1" {
		t.Fatalf("expected trimmed session ID, got %q", payload.SessionID)
	}
	if payload.ScenarioCode != "open-water-duel" {
		t.Fatalf("expected default scenario, got %q", payload.ScenarioCode)
	}
}

func TestAnalyticsServiceStartBattleRejectsInvalidPayloadBeforeUpstreamCall(t *testing.T) {
	cases := []struct {
		name string
		req  BattleSimulationStartRequest
	}{
		{
			name: "blank session",
			req:  BattleSimulationStartRequest{SessionID: "   ", ScenarioCode: "open-water-duel", OriginLongitude: 121.49, OriginLatitude: 31.23},
		},
		{
			name: "unknown scenario",
			req:  BattleSimulationStartRequest{SessionID: "battle-1", ScenarioCode: "missing-scenario", OriginLongitude: 121.49, OriginLatitude: 31.23},
		},
		{
			name: "invalid longitude",
			req:  BattleSimulationStartRequest{SessionID: "battle-1", ScenarioCode: "open-water-duel", OriginLongitude: 181, OriginLatitude: 31.23},
		},
		{
			name: "invalid latitude",
			req:  BattleSimulationStartRequest{SessionID: "battle-1", ScenarioCode: "open-water-duel", OriginLongitude: 121.49, OriginLatitude: -91},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()

			service := NewAnalyticsService(config.Config{AnalyticsBaseURL: server.URL})
			_, err := service.StartBattleSimulation(context.Background(), tc.req)

			if !IsValidationError(err) {
				t.Fatalf("expected validation error, got %v", err)
			}
			if called {
				t.Fatal("expected invalid battle simulation payload to be rejected before calling analytics")
			}
		})
	}
}

func TestAnalyticsServiceStartSimulationDefaultsShipIDs(t *testing.T) {
	var gotShipIDs []uint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/simulate/start" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var payload SimulationStartRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		gotShipIDs = payload.ShipIDs
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"running": true})
	}))
	defer server.Close()

	service := NewAnalyticsService(config.Config{AnalyticsBaseURL: server.URL})
	if _, err := service.StartSimulation(context.Background(), SimulationStartRequest{}); err != nil {
		t.Fatalf("StartSimulation returned error: %v", err)
	}
	if len(gotShipIDs) != 2 || gotShipIDs[0] != 1 || gotShipIDs[1] != 2 {
		t.Fatalf("expected default ship IDs [1 2], got %#v", gotShipIDs)
	}
}

func TestAnalyticsServiceStartSimulationRejectsZeroShipIDBeforeUpstreamCall(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	service := NewAnalyticsService(config.Config{AnalyticsBaseURL: server.URL})
	_, err := service.StartSimulation(context.Background(), SimulationStartRequest{ShipIDs: []uint{1, 0}})

	if !IsValidationError(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if called {
		t.Fatal("expected invalid ship IDs to be rejected before calling analytics")
	}
}

func TestAnalyticsServiceStopBattleTrimsSessionID(t *testing.T) {
	var payload BattleSimulationStopRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/simulate/battle/stop" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"running": false})
	}))
	defer server.Close()

	service := NewAnalyticsService(config.Config{AnalyticsBaseURL: server.URL})
	if _, err := service.StopBattleSimulation(context.Background(), BattleSimulationStopRequest{SessionID: " battle-1 "}); err != nil {
		t.Fatalf("StopBattleSimulation returned error: %v", err)
	}
	if payload.SessionID != "battle-1" {
		t.Fatalf("expected trimmed session ID, got %q", payload.SessionID)
	}
}

func TestAnalyticsServiceReturnsErrorForBadStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"detail": "denied"})
	}))
	defer server.Close()

	service := NewAnalyticsService(config.Config{AnalyticsBaseURL: server.URL})
	if _, err := service.Status(context.Background()); err == nil {
		t.Fatal("expected non-2xx analytics response to return an error")
	}
}

func TestAnalyticsServiceReturnsStructuredUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(observability.RequestIDHeader, "trace-upstream")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"message":   "request validation failed",
			"requestId": "trace-body",
		})
	}))
	defer server.Close()

	service := NewAnalyticsService(config.Config{AnalyticsBaseURL: server.URL})
	_, err := service.Status(context.Background())
	if err == nil {
		t.Fatal("expected non-2xx analytics response to return an error")
	}
	analyticsErr, ok := AsAnalyticsError(err)
	if !ok {
		t.Fatalf("expected AnalyticsError, got %T %v", err, err)
	}
	if analyticsErr.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected upstream status 422, got %d", analyticsErr.StatusCode)
	}
	if analyticsErr.Message != "request validation failed" {
		t.Fatalf("expected upstream message, got %q", analyticsErr.Message)
	}
	if analyticsErr.RequestID != "trace-upstream" {
		t.Fatalf("expected upstream request id from header, got %q", analyticsErr.RequestID)
	}
}

func TestAnalyticsServiceFallsBackToBodyRequestIDWhenHeaderIsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"message":   "payload rejected",
			"requestId": "trace-body-only",
		})
	}))
	defer server.Close()

	service := NewAnalyticsService(config.Config{AnalyticsBaseURL: server.URL})
	_, err := service.Status(context.Background())
	if err == nil {
		t.Fatal("expected non-2xx analytics response to return an error")
	}
	analyticsErr, ok := AsAnalyticsError(err)
	if !ok {
		t.Fatalf("expected AnalyticsError, got %T %v", err, err)
	}
	if analyticsErr.RequestID != "trace-body-only" {
		t.Fatalf("expected requestId to fall back to response body, got %q", analyticsErr.RequestID)
	}
}

func TestAnalyticsServicePreservesNonJSONErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream analytics unavailable"))
	}))
	defer server.Close()

	service := NewAnalyticsService(config.Config{AnalyticsBaseURL: server.URL})
	_, err := service.Status(context.Background())
	if err == nil {
		t.Fatal("expected non-2xx analytics response to return an error")
	}
	message := err.Error()
	if !strings.Contains(message, "status=502") || !strings.Contains(message, "upstream analytics unavailable") {
		t.Fatalf("expected status and body summary in error, got %q", message)
	}
}

func TestAnalyticsServiceAcceptsEmptySuccessBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	service := NewAnalyticsService(config.Config{AnalyticsBaseURL: server.URL})
	data, err := service.Status(context.Background())
	if err != nil {
		t.Fatalf("expected empty success body to be accepted, got %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("expected empty result map, got %#v", data)
	}
}

func TestAnalyticsServiceRejectsOversizedSuccessBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("x", maxAnalyticsResponseBytes+1)))
	}))
	defer server.Close()

	service := NewAnalyticsService(config.Config{AnalyticsBaseURL: server.URL})
	_, err := service.Status(context.Background())
	if err == nil {
		t.Fatal("expected oversized success body to return an error")
	}
	if !strings.Contains(err.Error(), "analytics response body exceeds") {
		t.Fatalf("expected oversized body error, got %q", err.Error())
	}
}
