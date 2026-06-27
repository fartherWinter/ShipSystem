package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"shipsystem/backend/internal/config"
	"shipsystem/backend/internal/observability"
)

type AnalyticsService struct {
	baseURL    string
	adminToken string
	client     *http.Client
}

const maxAnalyticsResponseBytes = 1 << 20

type AnalyticsError struct {
	StatusCode int
	Message    string
	RequestID  string
	Body       string
}

func (e *AnalyticsError) Error() string {
	message := e.Message
	if message == "" {
		message = e.Body
	}
	if message == "" {
		message = http.StatusText(e.StatusCode)
	}
	if e.RequestID != "" {
		return fmt.Sprintf("analytics request failed: status=%d request_id=%s message=%s", e.StatusCode, e.RequestID, message)
	}
	return fmt.Sprintf("analytics request failed: status=%d message=%s", e.StatusCode, message)
}

func AsAnalyticsError(err error) (*AnalyticsError, bool) {
	var analyticsErr *AnalyticsError
	if errors.As(err, &analyticsErr) {
		return analyticsErr, true
	}
	return nil, false
}

type SimulationStartRequest struct {
	ShipIDs         []uint  `json:"shipIds"`
	OriginLongitude float64 `json:"originLongitude,omitempty"`
	OriginLatitude  float64 `json:"originLatitude,omitempty"`
}

type BattleSimulationStartRequest struct {
	SessionID       string  `json:"sessionId"`
	ScenarioCode    string  `json:"scenarioCode"`
	OriginLongitude float64 `json:"originLongitude"`
	OriginLatitude  float64 `json:"originLatitude"`
	Seed            int     `json:"seed,omitempty"`
}

type BattleSimulationStopRequest struct {
	SessionID string `json:"sessionId"`
}

func NewAnalyticsService(cfg config.Config) *AnalyticsService {
	timeout := cfg.AnalyticsHTTPTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &AnalyticsService{
		baseURL:    cfg.AnalyticsBaseURL,
		adminToken: cfg.AnalyticsAdminToken,
		client:     &http.Client{Timeout: timeout},
	}
}

func (s *AnalyticsService) Status(ctx context.Context) (map[string]interface{}, error) {
	return s.do(ctx, http.MethodGet, "/simulate/status", nil)
}

func (s *AnalyticsService) StartSimulation(ctx context.Context, req SimulationStartRequest) (map[string]interface{}, error) {
	if len(req.ShipIDs) == 0 {
		req.ShipIDs = []uint{1, 2}
	}
	for _, shipID := range req.ShipIDs {
		if shipID == 0 {
			return nil, &ValidationError{message: "shipIds must contain only positive IDs"}
		}
	}
	return s.do(ctx, http.MethodPost, "/simulate/start", req)
}

func (s *AnalyticsService) StopSimulation(ctx context.Context) (map[string]interface{}, error) {
	return s.do(ctx, http.MethodPost, "/simulate/stop", map[string]interface{}{})
}

func (s *AnalyticsService) StartBattleSimulation(ctx context.Context, req BattleSimulationStartRequest) (map[string]interface{}, error) {
	req.SessionID = strings.TrimSpace(req.SessionID)
	req.ScenarioCode = strings.TrimSpace(req.ScenarioCode)
	if req.SessionID == "" {
		return nil, &ValidationError{message: "sessionId is required"}
	}
	if req.ScenarioCode == "" {
		req.ScenarioCode = "open-water-duel"
	}
	if _, err := NewAppService(nil, nil).findScenario(req.ScenarioCode); err != nil {
		return nil, err
	}
	if !validLongitude(req.OriginLongitude) || !validLatitude(req.OriginLatitude) {
		return nil, &ValidationError{message: "battle simulation origin coordinates are invalid"}
	}
	return s.do(ctx, http.MethodPost, "/simulate/battle/start", req)
}

func (s *AnalyticsService) StopBattleSimulation(ctx context.Context, req BattleSimulationStopRequest) (map[string]interface{}, error) {
	req.SessionID = strings.TrimSpace(req.SessionID)
	if req.SessionID == "" {
		return nil, &ValidationError{message: "sessionId is required"}
	}
	return s.do(ctx, http.MethodPost, "/simulate/battle/stop", req)
}

func (s *AnalyticsService) do(ctx context.Context, method, path string, payload interface{}) (map[string]interface{}, error) {
	if s.baseURL == "" {
		return nil, &ValidationError{message: "analytics service is not configured"}
	}

	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if requestID := observability.RequestIDFromContext(ctx); requestID != "" {
		req.Header.Set(observability.RequestIDHeader, requestID)
	}
	if s.adminToken != "" {
		req.Header.Set("X-Analytics-Token", s.adminToken)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	result, rawBody, err := decodeAnalyticsResponse(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, newAnalyticsError(resp, result, rawBody, err)
	}
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = map[string]interface{}{}
	}
	return result, nil
}

func newAnalyticsError(resp *http.Response, result map[string]interface{}, rawBody []byte, decodeErr error) *AnalyticsError {
	body := summarizeAnalyticsBody(rawBody)
	message := analyticsErrorMessage(result, body, decodeErr)
	requestID := strings.TrimSpace(resp.Header.Get(observability.RequestIDHeader))
	if requestID == "" && result != nil {
		if value, ok := result["requestId"].(string); ok {
			requestID = strings.TrimSpace(value)
		}
	}
	return &AnalyticsError{
		StatusCode: resp.StatusCode,
		Message:    message,
		RequestID:  requestID,
		Body:       body,
	}
}

func analyticsErrorMessage(result map[string]interface{}, body string, decodeErr error) string {
	if result != nil {
		if value, ok := result["message"].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
		if value, ok := result["detail"].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	if body != "" && body != "<empty>" {
		return body
	}
	if decodeErr != nil {
		return decodeErr.Error()
	}
	return ""
}

func decodeAnalyticsResponse(body io.Reader) (map[string]interface{}, []byte, error) {
	rawBody, err := io.ReadAll(io.LimitReader(body, maxAnalyticsResponseBytes+1))
	if err != nil {
		return nil, rawBody, err
	}
	if len(rawBody) > maxAnalyticsResponseBytes {
		return nil, rawBody[:maxAnalyticsResponseBytes], fmt.Errorf("analytics response body exceeds %d bytes", maxAnalyticsResponseBytes)
	}
	if len(bytes.TrimSpace(rawBody)) == 0 {
		return nil, rawBody, nil
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rawBody, &result); err != nil {
		return nil, rawBody, err
	}
	return result, rawBody, nil
}

func summarizeAnalyticsBody(rawBody []byte) string {
	text := strings.TrimSpace(string(rawBody))
	if text == "" {
		return "<empty>"
	}
	const maxSummaryLength = 300
	if len(text) > maxSummaryLength {
		return text[:maxSummaryLength] + "..."
	}
	return text
}
