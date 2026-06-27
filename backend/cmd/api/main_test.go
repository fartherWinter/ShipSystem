package main

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"shipsystem/backend/internal/config"
	"shipsystem/backend/internal/handlers"
	"shipsystem/backend/internal/middleware"
	"shipsystem/backend/internal/models"
	"shipsystem/backend/internal/services"
)

func TestEmbeddedTZDataSupportsAsiaShanghai(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("expected Asia/Shanghai location to load, got %v", err)
	}
	if location.String() != "Asia/Shanghai" {
		t.Fatalf("expected Asia/Shanghai location, got %s", location.String())
	}
}

func TestNewHTTPServerUsesConfiguredTimeouts(t *testing.T) {
	server := newHTTPServer(config.Config{
		Port:              "9090",
		HTTPReadTimeout:   21 * time.Second,
		HTTPHeaderTimeout: 7 * time.Second,
		HTTPWriteTimeout:  31 * time.Second,
		HTTPIdleTimeout:   61 * time.Second,
	}, http.NewServeMux())

	if server.Addr != ":9090" {
		t.Fatalf("expected addr :9090, got %s", server.Addr)
	}
	if server.ReadTimeout != 21*time.Second {
		t.Fatalf("expected read timeout 21s, got %s", server.ReadTimeout)
	}
	if server.ReadHeaderTimeout != 7*time.Second {
		t.Fatalf("expected read header timeout 7s, got %s", server.ReadHeaderTimeout)
	}
	if server.WriteTimeout != 31*time.Second {
		t.Fatalf("expected write timeout 31s, got %s", server.WriteTimeout)
	}
	if server.IdleTimeout != 61*time.Second {
		t.Fatalf("expected idle timeout 61s, got %s", server.IdleTimeout)
	}
}

func TestRequestBodyLimitMiddlewareRejectsOversizedCreateShipPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Config{
		Port:             "8080",
		RequestBodyLimit: 1024,
		CORSOrigins:      []string{"http://localhost:5173"},
		JWTSecret:        "0123456789abcdef0123456789abcdef",
	}
	router := gin.New()
	router.Use(middleware.RequestID())
	router.Use(middleware.Recovery(log.New(io.Discard, "", 0)))
	router.Use(middleware.RequestBodyLimit(cfg.RequestBodyLimit))

	handler := handlers.NewHandler(nil, noopAppService{}, nil, nil, 0, false, cfg.CORSOrigins)
	router.POST("/api/v1/ships", handler.CreateShip)

	reqBody := `{"name":"ship","mmsi":"` + strings.Repeat("9", 1500) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ships", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(middleware.RequestIDHeader, "trace-main-body-limit")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "request entity too large") {
		t.Fatalf("expected body limit error, got %s", recorder.Body.String())
	}
}

type noopAppService struct{}

func (noopAppService) ListShips(ctx context.Context, keyword string, page, size int) ([]models.Ship, int64, error) {
	return nil, 0, nil
}

func (noopAppService) CreateShip(ctx context.Context, ship *models.Ship) error {
	return nil
}

func (noopAppService) GetShip(ctx context.Context, id uint) (models.Ship, error) {
	return models.Ship{}, nil
}

func (noopAppService) UpdateShip(ctx context.Context, id uint, patch models.Ship) (models.Ship, error) {
	return models.Ship{}, nil
}

func (noopAppService) DeleteShip(ctx context.Context, id uint) error {
	return nil
}

func (noopAppService) ReportLocation(ctx context.Context, loc *models.ShipLocation) (*models.Alarm, error) {
	return nil, nil
}

func (noopAppService) ListTracks(ctx context.Context, shipID uint, start, end *time.Time) ([]models.ShipLocation, error) {
	return nil, nil
}

func (noopAppService) ListAlarms(ctx context.Context, status string, page, size int) ([]models.Alarm, int64, error) {
	return nil, 0, nil
}

func (noopAppService) AckAlarm(ctx context.Context, id, userID uint) (models.Alarm, error) {
	return models.Alarm{}, nil
}

func (noopAppService) ListDispatchEvents(ctx context.Context, status string, page, size int) ([]models.DispatchEvent, int64, error) {
	return nil, 0, nil
}

func (noopAppService) CreateDispatchEvent(ctx context.Context, event *models.DispatchEvent) error {
	return nil
}

func (noopAppService) UpdateDispatchStatus(ctx context.Context, id uint, toStatus, remark string, operatorID *uint) (models.DispatchEvent, error) {
	return models.DispatchEvent{}, nil
}

func (noopAppService) ListUsers(ctx context.Context) ([]models.User, error) {
	return nil, nil
}

func (noopAppService) ListRoles(ctx context.Context) ([]models.Role, error) {
	return nil, nil
}

func (noopAppService) ListMenus(ctx context.Context) ([]models.Menu, error) {
	return nil, nil
}

func (noopAppService) ListBattleScenarios(ctx context.Context) []services.BattleScenario {
	return nil
}

func (noopAppService) ListBattleSessions(ctx context.Context, page, size int) ([]models.BattleSession, int64, error) {
	return nil, 0, nil
}

func (noopAppService) CreateBattleSession(ctx context.Context, scenarioCode string) (models.BattleSession, services.BattleStateSnapshot, error) {
	return models.BattleSession{}, services.BattleStateSnapshot{}, nil
}

func (noopAppService) GetBattleState(ctx context.Context, sessionID string) (services.BattleStateSnapshot, error) {
	return services.BattleStateSnapshot{}, nil
}

func (noopAppService) ListBattleTimeline(ctx context.Context, sessionID string) ([]services.BattleTimelineItem, error) {
	return nil, nil
}

func (noopAppService) ListBattleSnapshots(ctx context.Context, sessionID string, fromTick, toTick *int64) ([]services.BattleSnapshotView, error) {
	return nil, nil
}

func (noopAppService) GetBattleReport(ctx context.Context, sessionID string) (services.BattleReport, error) {
	return services.BattleReport{}, nil
}

func (noopAppService) StopBattleSession(ctx context.Context, sessionID string) (services.BattleStateSnapshot, error) {
	return services.BattleStateSnapshot{}, nil
}

func (noopAppService) ReceiveRadarReport(ctx context.Context, report services.RadarReportPayload) (services.BattleStateSnapshot, error) {
	return services.BattleStateSnapshot{}, nil
}
