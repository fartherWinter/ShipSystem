package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shipsystem/backend/internal/config"
	"shipsystem/backend/internal/middleware"
	"shipsystem/backend/internal/models"
	"shipsystem/backend/internal/services"
)

type fakeAppService struct {
	createShipFn           func(ctx context.Context, ship *models.Ship) error
	listShipsFn            func(ctx context.Context, keyword string, page, size int) ([]models.Ship, int64, error)
	getShipFn              func(ctx context.Context, id uint) (models.Ship, error)
	updateShipFn           func(ctx context.Context, id uint, patch models.Ship) (models.Ship, error)
	deleteShipFn           func(ctx context.Context, id uint) error
	reportLocationFn       func(ctx context.Context, loc *models.ShipLocation) (*models.Alarm, error)
	listTracksFn           func(ctx context.Context, shipID uint, start, end *time.Time) ([]models.ShipLocation, error)
	listAlarmsFn           func(ctx context.Context, status string, page, size int) ([]models.Alarm, int64, error)
	ackAlarmFn             func(ctx context.Context, id, userID uint) (models.Alarm, error)
	createDispatchEventFn  func(ctx context.Context, event *models.DispatchEvent) error
	listDispatchEventsFn   func(ctx context.Context, status string, page, size int) ([]models.DispatchEvent, int64, error)
	updateDispatchStatusFn func(ctx context.Context, id uint, toStatus, remark string, operatorID *uint) (models.DispatchEvent, error)
	createBattleSessionFn  func(ctx context.Context, scenarioCode string) (models.BattleSession, services.BattleStateSnapshot, error)
	listBattleSessionsFn   func(ctx context.Context, page, size int) ([]models.BattleSession, int64, error)
	listUsersFn            func(ctx context.Context) ([]models.User, error)
	listRolesFn            func(ctx context.Context) ([]models.Role, error)
	listMenusFn            func(ctx context.Context) ([]models.Menu, error)
	getBattleStateFn       func(ctx context.Context, sessionID string) (services.BattleStateSnapshot, error)
	listBattleTimelineFn   func(ctx context.Context, sessionID string) ([]services.BattleTimelineItem, error)
	listBattleSnapshotsFn  func(ctx context.Context, sessionID string, fromTick, toTick *int64) ([]services.BattleSnapshotView, error)
	getBattleReportFn      func(ctx context.Context, sessionID string) (services.BattleReport, error)
	stopBattleSessionFn    func(ctx context.Context, sessionID string) (services.BattleStateSnapshot, error)
	receiveRadarReportFn   func(ctx context.Context, report services.RadarReportPayload) (services.BattleStateSnapshot, error)
}

type fakeAnalyticsService struct {
	statusFn                func(ctx context.Context) (map[string]interface{}, error)
	startSimulationFn       func(ctx context.Context, req services.SimulationStartRequest) (map[string]interface{}, error)
	stopSimulationFn        func(ctx context.Context) (map[string]interface{}, error)
	startBattleSimulationFn func(ctx context.Context, req services.BattleSimulationStartRequest) (map[string]interface{}, error)
	stopBattleSimulationFn  func(ctx context.Context, req services.BattleSimulationStopRequest) (map[string]interface{}, error)
}

type fakeAuthService struct {
	loginFn      func(ctx context.Context, username, password string) (string, models.User, []models.Menu, error)
	parseTokenFn func(tokenText string) (*services.Claims, error)
}

var _ authService = (*fakeAuthService)(nil)
var _ appService = (*fakeAppService)(nil)
var _ analyticsService = (*fakeAnalyticsService)(nil)

func (f *fakeAuthService) Login(ctx context.Context, username, password string) (string, models.User, []models.Menu, error) {
	if f.loginFn != nil {
		return f.loginFn(ctx, username, password)
	}
	return "", models.User{}, nil, nil
}

func (f *fakeAuthService) ParseToken(tokenText string) (*services.Claims, error) {
	if f.parseTokenFn != nil {
		return f.parseTokenFn(tokenText)
	}
	return &services.Claims{}, nil
}

func (f *fakeAppService) ListShips(ctx context.Context, keyword string, page, size int) ([]models.Ship, int64, error) {
	if f.listShipsFn != nil {
		return f.listShipsFn(ctx, keyword, page, size)
	}
	return nil, 0, nil
}

func (f *fakeAppService) CreateShip(ctx context.Context, ship *models.Ship) error {
	if f.createShipFn != nil {
		return f.createShipFn(ctx, ship)
	}
	return nil
}

func (f *fakeAppService) GetShip(ctx context.Context, id uint) (models.Ship, error) {
	if f.getShipFn != nil {
		return f.getShipFn(ctx, id)
	}
	return models.Ship{}, nil
}

func (f *fakeAppService) UpdateShip(ctx context.Context, id uint, patch models.Ship) (models.Ship, error) {
	if f.updateShipFn != nil {
		return f.updateShipFn(ctx, id, patch)
	}
	return models.Ship{}, nil
}

func (f *fakeAppService) DeleteShip(ctx context.Context, id uint) error {
	if f.deleteShipFn != nil {
		return f.deleteShipFn(ctx, id)
	}
	return nil
}

func (f *fakeAppService) ReportLocation(ctx context.Context, loc *models.ShipLocation) (*models.Alarm, error) {
	if f.reportLocationFn != nil {
		return f.reportLocationFn(ctx, loc)
	}
	return nil, nil
}

func (f *fakeAppService) ListTracks(ctx context.Context, shipID uint, start, end *time.Time) ([]models.ShipLocation, error) {
	if f.listTracksFn != nil {
		return f.listTracksFn(ctx, shipID, start, end)
	}
	return nil, nil
}

func (f *fakeAppService) ListAlarms(ctx context.Context, status string, page, size int) ([]models.Alarm, int64, error) {
	if f.listAlarmsFn != nil {
		return f.listAlarmsFn(ctx, status, page, size)
	}
	return nil, 0, nil
}

func (f *fakeAppService) AckAlarm(ctx context.Context, id, userID uint) (models.Alarm, error) {
	if f.ackAlarmFn != nil {
		return f.ackAlarmFn(ctx, id, userID)
	}
	return models.Alarm{}, nil
}

func (f *fakeAppService) ListDispatchEvents(ctx context.Context, status string, page, size int) ([]models.DispatchEvent, int64, error) {
	if f.listDispatchEventsFn != nil {
		return f.listDispatchEventsFn(ctx, status, page, size)
	}
	return nil, 0, nil
}

func (f *fakeAppService) CreateDispatchEvent(ctx context.Context, event *models.DispatchEvent) error {
	if f.createDispatchEventFn != nil {
		return f.createDispatchEventFn(ctx, event)
	}
	return nil
}

func (f *fakeAppService) UpdateDispatchStatus(ctx context.Context, id uint, toStatus, remark string, operatorID *uint) (models.DispatchEvent, error) {
	if f.updateDispatchStatusFn != nil {
		return f.updateDispatchStatusFn(ctx, id, toStatus, remark, operatorID)
	}
	return models.DispatchEvent{}, nil
}

func (f *fakeAppService) ListUsers(ctx context.Context) ([]models.User, error) {
	if f.listUsersFn != nil {
		return f.listUsersFn(ctx)
	}
	return nil, nil
}

func (f *fakeAppService) ListRoles(ctx context.Context) ([]models.Role, error) {
	if f.listRolesFn != nil {
		return f.listRolesFn(ctx)
	}
	return nil, nil
}

func (f *fakeAppService) ListMenus(ctx context.Context) ([]models.Menu, error) {
	if f.listMenusFn != nil {
		return f.listMenusFn(ctx)
	}
	return nil, nil
}

func (f *fakeAppService) ListBattleScenarios(ctx context.Context) []services.BattleScenario {
	return nil
}

func (f *fakeAppService) ListBattleSessions(ctx context.Context, page, size int) ([]models.BattleSession, int64, error) {
	if f.listBattleSessionsFn != nil {
		return f.listBattleSessionsFn(ctx, page, size)
	}
	return nil, 0, nil
}

func (f *fakeAppService) CreateBattleSession(ctx context.Context, scenarioCode string) (models.BattleSession, services.BattleStateSnapshot, error) {
	if f.createBattleSessionFn != nil {
		return f.createBattleSessionFn(ctx, scenarioCode)
	}
	return models.BattleSession{}, services.BattleStateSnapshot{}, nil
}

func (f *fakeAppService) GetBattleState(ctx context.Context, sessionID string) (services.BattleStateSnapshot, error) {
	if f.getBattleStateFn != nil {
		return f.getBattleStateFn(ctx, sessionID)
	}
	return services.BattleStateSnapshot{}, nil
}

func (f *fakeAppService) ListBattleTimeline(ctx context.Context, sessionID string) ([]services.BattleTimelineItem, error) {
	if f.listBattleTimelineFn != nil {
		return f.listBattleTimelineFn(ctx, sessionID)
	}
	return nil, nil
}

func (f *fakeAppService) ListBattleSnapshots(ctx context.Context, sessionID string, fromTick, toTick *int64) ([]services.BattleSnapshotView, error) {
	if f.listBattleSnapshotsFn != nil {
		return f.listBattleSnapshotsFn(ctx, sessionID, fromTick, toTick)
	}
	return nil, nil
}

func (f *fakeAppService) GetBattleReport(ctx context.Context, sessionID string) (services.BattleReport, error) {
	if f.getBattleReportFn != nil {
		return f.getBattleReportFn(ctx, sessionID)
	}
	return services.BattleReport{}, nil
}

func (f *fakeAppService) StopBattleSession(ctx context.Context, sessionID string) (services.BattleStateSnapshot, error) {
	if f.stopBattleSessionFn != nil {
		return f.stopBattleSessionFn(ctx, sessionID)
	}
	return services.BattleStateSnapshot{}, nil
}

func (f *fakeAppService) ReceiveRadarReport(ctx context.Context, report services.RadarReportPayload) (services.BattleStateSnapshot, error) {
	if f.receiveRadarReportFn != nil {
		return f.receiveRadarReportFn(ctx, report)
	}
	return services.BattleStateSnapshot{}, nil
}

func (f *fakeAnalyticsService) Status(ctx context.Context) (map[string]interface{}, error) {
	if f.statusFn != nil {
		return f.statusFn(ctx)
	}
	return map[string]interface{}{}, nil
}

func (f *fakeAnalyticsService) StartSimulation(ctx context.Context, req services.SimulationStartRequest) (map[string]interface{}, error) {
	if f.startSimulationFn != nil {
		return f.startSimulationFn(ctx, req)
	}
	return map[string]interface{}{}, nil
}

func (f *fakeAnalyticsService) StopSimulation(ctx context.Context) (map[string]interface{}, error) {
	if f.stopSimulationFn != nil {
		return f.stopSimulationFn(ctx)
	}
	return map[string]interface{}{}, nil
}

func (f *fakeAnalyticsService) StartBattleSimulation(ctx context.Context, req services.BattleSimulationStartRequest) (map[string]interface{}, error) {
	if f.startBattleSimulationFn != nil {
		return f.startBattleSimulationFn(ctx, req)
	}
	return map[string]interface{}{}, nil
}

func (f *fakeAnalyticsService) StopBattleSimulation(ctx context.Context, req services.BattleSimulationStopRequest) (map[string]interface{}, error) {
	if f.stopBattleSimulationFn != nil {
		return f.stopBattleSimulationFn(ctx, req)
	}
	return map[string]interface{}{}, nil
}

func TestIDParamRejectsZeroAndInvalidValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, value := range []string{"0", "abc", "-1"} {
		t.Run(value, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Params = gin.Params{{Key: "id", Value: value}}
			if id, ok := idParam(ctx); ok || id != 0 {
				t.Fatalf("expected invalid id, got id=%d ok=%v", id, ok)
			}
		})
	}
}

func TestIDParamAcceptsPositiveID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Params = gin.Params{{Key: "id", Value: "42"}}

	id, ok := idParam(ctx)
	if !ok || id != 42 {
		t.Fatalf("expected id 42, got id=%d ok=%v", id, ok)
	}
}

func TestParseTimeRejectsInvalidValue(t *testing.T) {
	if _, err := parseTime("not-a-time"); err == nil {
		t.Fatal("expected invalid time to be rejected")
	}
}

func TestTickQueryRejectsNegativeValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/battle/sessions/test/snapshots?from=-1", nil)

	if _, err := tickQuery(ctx, "from"); err == nil {
		t.Fatal("expected negative tick to be rejected")
	}
}

func TestCoordinateValidation(t *testing.T) {
	if !validLongitude(121.49) || validLongitude(181) {
		t.Fatal("longitude validation failed")
	}
	if !validLatitude(31.23) || validLatitude(-91) {
		t.Fatal("latitude validation failed")
	}
}

func TestPaginationDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/ships", nil)

	page, size, ok := pagination(ctx)
	if !ok {
		t.Fatal("expected default pagination to be valid")
	}
	if page != 1 || size != 20 {
		t.Fatalf("expected default page=1 size=20, got page=%d size=%d", page, size)
	}
}

func TestPaginationRejectsInvalidPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/ships?page=abc", nil)
	ctx.Set(middleware.RequestIDKey, "trace-pagination")

	if _, _, ok := pagination(ctx); ok {
		t.Fatal("expected invalid page to be rejected")
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["requestId"] != "trace-pagination" {
		t.Fatalf("expected requestId trace-pagination, got %#v", body)
	}
}

func TestPaginationRejectsOversizedPageSize(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/ships?size=101", nil)

	if _, _, ok := pagination(ctx); ok {
		t.Fatal("expected oversized size to be rejected")
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestListBattleSessionsReturnsBadRequestForInvalidPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, nil, nil, nil, 0, false, nil)
	router := gin.New()
	router.GET("/battle/sessions", handler.ListBattleSessions)

	req := httptest.NewRequest(http.MethodGet, "/battle/sessions?size=101", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestListShipsReturnsInternalServerErrorWithoutLeakingStorageError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		listShipsFn: func(ctx context.Context, keyword string, page, size int) ([]models.Ship, int64, error) {
			return nil, 0, io.ErrUnexpectedEOF
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.GET("/ships", handler.ListShips)

	req := httptest.NewRequest(http.MethodGet, "/ships", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "failed to list ships" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestGetShipReturnsNotFoundWhenShipDoesNotExist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		getShipFn: func(ctx context.Context, id uint) (models.Ship, error) {
			return models.Ship{}, gorm.ErrRecordNotFound
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.GET("/ships/:id", handler.GetShip)

	req := httptest.NewRequest(http.MethodGet, "/ships/42", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "ship does not exist" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestGetShipReturnsInternalServerErrorWithoutLeakingStorageError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		getShipFn: func(ctx context.Context, id uint) (models.Ship, error) {
			return models.Ship{}, io.ErrUnexpectedEOF
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.GET("/ships/:id", handler.GetShip)

	req := httptest.NewRequest(http.MethodGet, "/ships/42", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "failed to get ship" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestListBattleSessionsReturnsInternalServerErrorWithoutLeakingStorageError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		listBattleSessionsFn: func(ctx context.Context, page, size int) ([]models.BattleSession, int64, error) {
			return nil, 0, io.ErrUnexpectedEOF
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.GET("/battle/sessions", handler.ListBattleSessions)

	req := httptest.NewRequest(http.MethodGet, "/battle/sessions", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "failed to list battle sessions" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestListUsersReturnsInternalServerErrorWithoutLeakingStorageError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		listUsersFn: func(ctx context.Context) ([]models.User, error) {
			return nil, io.ErrUnexpectedEOF
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.GET("/rbac/users", handler.ListUsers)

	req := httptest.NewRequest(http.MethodGet, "/rbac/users", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "failed to list users" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestListRolesReturnsInternalServerErrorWithoutLeakingStorageError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		listRolesFn: func(ctx context.Context) ([]models.Role, error) {
			return nil, io.ErrUnexpectedEOF
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.GET("/rbac/roles", handler.ListRoles)

	req := httptest.NewRequest(http.MethodGet, "/rbac/roles", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "failed to list roles" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestListMenusReturnsInternalServerErrorWithoutLeakingStorageError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		listMenusFn: func(ctx context.Context) ([]models.Menu, error) {
			return nil, io.ErrUnexpectedEOF
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.GET("/rbac/menus", handler.ListMenus)

	req := httptest.NewRequest(http.MethodGet, "/rbac/menus", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "failed to list menus" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestCreateShipReturnsValidationMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		createShipFn: func(ctx context.Context, ship *models.Ship) error {
			return services.NewValidationError("船名和 MMSI 必填")
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.POST("/ships", handler.CreateShip)

	req := httptest.NewRequest(http.MethodPost, "/ships", bytes.NewBufferString(`{"name":"Voyager","mmsi":"123456789"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "船名和 MMSI 必填" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestCreateShipReturnsInternalServerErrorWithoutLeakingStorageError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		createShipFn: func(ctx context.Context, ship *models.Ship) error {
			return io.ErrUnexpectedEOF
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.POST("/ships", handler.CreateShip)

	req := httptest.NewRequest(http.MethodPost, "/ships", bytes.NewBufferString(`{"name":"Voyager","mmsi":"123456789"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "failed to create ship" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestLoginReturnsUnauthorizedForCredentialFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(&fakeAuthService{
		loginFn: func(ctx context.Context, username, password string) (string, models.User, []models.Menu, error) {
			return "", models.User{}, nil, services.NewAuthenticationError("用户名或密码错误")
		},
	}, nil, nil, nil, 0, false, nil)
	router := gin.New()
	router.POST("/auth/login", handler.Login)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"username":"demo","password":"bad"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "用户名或密码错误" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestLoginReturnsInternalServerErrorWithoutLeakingInternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(&fakeAuthService{
		loginFn: func(ctx context.Context, username, password string) (string, models.User, []models.Menu, error) {
			return "", models.User{}, nil, io.ErrUnexpectedEOF
		},
	}, nil, nil, nil, 0, false, nil)
	router := gin.New()
	router.POST("/auth/login", handler.Login)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"username":"demo","password":"good"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "failed to complete login" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestListBattleSnapshotsReturnsBadRequestForInvalidTickRange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, nil, nil, nil, 0, false, nil)
	router := gin.New()
	router.GET("/battle/sessions/:sessionId/snapshots", handler.ListBattleSnapshots)

	req := httptest.NewRequest(http.MethodGet, "/battle/sessions/battle-1/snapshots?from=10&to=2", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestListTracksReturnsNotFoundWhenShipDoesNotExist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		listTracksFn: func(ctx context.Context, shipID uint, start, end *time.Time) ([]models.ShipLocation, error) {
			return nil, gorm.ErrRecordNotFound
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.GET("/ships/:id/tracks", handler.ListTracks)

	req := httptest.NewRequest(http.MethodGet, "/ships/42/tracks", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "ship does not exist" {
		t.Fatalf("expected track query error message, got %#v", body)
	}
}

func TestUpdateShipReturnsNotFoundWhenShipDoesNotExist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		updateShipFn: func(ctx context.Context, id uint, patch models.Ship) (models.Ship, error) {
			return models.Ship{}, gorm.ErrRecordNotFound
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.PUT("/ships/:id", handler.UpdateShip)

	req := httptest.NewRequest(http.MethodPut, "/ships/42", bytes.NewBufferString(`{"name":"Voyager","mmsi":"123456789"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "ship does not exist" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestDeleteShipReturnsNotFoundWhenShipDoesNotExist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		deleteShipFn: func(ctx context.Context, id uint) error {
			return gorm.ErrRecordNotFound
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.DELETE("/ships/:id", handler.DeleteShip)

	req := httptest.NewRequest(http.MethodDelete, "/ships/42", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "ship does not exist" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestReportLocationReturnsNotFoundWhenShipDoesNotExist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		reportLocationFn: func(ctx context.Context, loc *models.ShipLocation) (*models.Alarm, error) {
			return nil, gorm.ErrRecordNotFound
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.POST("/ships/:id/locations", handler.ReportLocation)

	req := httptest.NewRequest(http.MethodPost, "/ships/42/locations", bytes.NewBufferString(`{"longitude":120.1,"latitude":30.2,"speedKnots":12.5,"course":90}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "ship does not exist" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestAckAlarmReturnsNotFoundWhenAlarmDoesNotExist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		ackAlarmFn: func(ctx context.Context, id, userID uint) (models.Alarm, error) {
			return models.Alarm{}, gorm.ErrRecordNotFound
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.PUT("/alarms/:id/ack", handler.AckAlarm)

	req := httptest.NewRequest(http.MethodPut, "/alarms/9/ack", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "alarm does not exist" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestListAlarmsReturnsValidationMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		listAlarmsFn: func(ctx context.Context, status string, page, size int) ([]models.Alarm, int64, error) {
			return nil, 0, services.NewValidationError("告警状态不合法")
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.GET("/alarms", handler.ListAlarms)

	req := httptest.NewRequest(http.MethodGet, "/alarms?status=BROKEN", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "告警状态不合法" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestListAlarmsReturnsInternalServerErrorWithoutLeakingStorageError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		listAlarmsFn: func(ctx context.Context, status string, page, size int) ([]models.Alarm, int64, error) {
			return nil, 0, io.ErrUnexpectedEOF
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.GET("/alarms", handler.ListAlarms)

	req := httptest.NewRequest(http.MethodGet, "/alarms", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "failed to list alarms" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestUpdateDispatchStatusReturnsNotFoundWhenEventDoesNotExist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		updateDispatchStatusFn: func(ctx context.Context, id uint, toStatus, remark string, operatorID *uint) (models.DispatchEvent, error) {
			return models.DispatchEvent{}, gorm.ErrRecordNotFound
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.PUT("/dispatch-events/:id/status", handler.UpdateDispatchStatus)

	req := httptest.NewRequest(http.MethodPut, "/dispatch-events/9/status", bytes.NewBufferString(`{"status":"PROCESSING"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "dispatch event does not exist" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestListDispatchEventsReturnsValidationMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		listDispatchEventsFn: func(ctx context.Context, status string, page, size int) ([]models.DispatchEvent, int64, error) {
			return nil, 0, services.NewValidationError("调度事件状态不合法")
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.GET("/dispatch-events", handler.ListDispatchEvents)

	req := httptest.NewRequest(http.MethodGet, "/dispatch-events?status=BROKEN", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "调度事件状态不合法" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestListDispatchEventsReturnsInternalServerErrorWithoutLeakingStorageError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		listDispatchEventsFn: func(ctx context.Context, status string, page, size int) ([]models.DispatchEvent, int64, error) {
			return nil, 0, io.ErrUnexpectedEOF
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.GET("/dispatch-events", handler.ListDispatchEvents)

	req := httptest.NewRequest(http.MethodGet, "/dispatch-events", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "failed to list dispatch events" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestCreateDispatchEventReturnsValidationMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		createDispatchEventFn: func(ctx context.Context, event *models.DispatchEvent) error {
			return services.NewValidationError("调度事件标题必填")
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.POST("/dispatch-events", handler.CreateDispatchEvent)

	req := httptest.NewRequest(http.MethodPost, "/dispatch-events", bytes.NewBufferString(`{"title":"demo"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "调度事件标题必填" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestCreateDispatchEventReturnsInternalServerErrorWithoutLeakingStorageError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		createDispatchEventFn: func(ctx context.Context, event *models.DispatchEvent) error {
			return io.ErrUnexpectedEOF
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.POST("/dispatch-events", handler.CreateDispatchEvent)

	req := httptest.NewRequest(http.MethodPost, "/dispatch-events", bytes.NewBufferString(`{"title":"demo"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "failed to create dispatch event" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestGetBattleTimelineReturnsNotFoundWhenSessionDoesNotExist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		listBattleTimelineFn: func(ctx context.Context, sessionID string) ([]services.BattleTimelineItem, error) {
			return nil, gorm.ErrRecordNotFound
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.GET("/battle/sessions/:sessionId/timeline", handler.GetBattleTimeline)

	req := httptest.NewRequest(http.MethodGet, "/battle/sessions/missing/timeline", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "battle session does not exist" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestCreateBattleSessionReturnsValidationMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		createBattleSessionFn: func(ctx context.Context, scenarioCode string) (models.BattleSession, services.BattleStateSnapshot, error) {
			return models.BattleSession{}, services.BattleStateSnapshot{}, services.NewValidationError("battle scenario does not exist")
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.POST("/battle/sessions", handler.CreateBattleSession)

	req := httptest.NewRequest(http.MethodPost, "/battle/sessions", bytes.NewBufferString(`{"scenarioCode":"missing"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "battle scenario does not exist" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestGetBattleStateReturnsNotFoundWhenSessionDoesNotExist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		getBattleStateFn: func(ctx context.Context, sessionID string) (services.BattleStateSnapshot, error) {
			return services.BattleStateSnapshot{}, gorm.ErrRecordNotFound
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.GET("/battle/sessions/:sessionId/state", handler.GetBattleState)

	req := httptest.NewRequest(http.MethodGet, "/battle/sessions/missing/state", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "battle session does not exist" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestGetBattleStateReturnsInternalServerErrorWithoutLeakingStorageError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		getBattleStateFn: func(ctx context.Context, sessionID string) (services.BattleStateSnapshot, error) {
			return services.BattleStateSnapshot{}, io.ErrUnexpectedEOF
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.GET("/battle/sessions/:sessionId/state", handler.GetBattleState)

	req := httptest.NewRequest(http.MethodGet, "/battle/sessions/session-1/state", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "failed to load battle state" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestListBattleSnapshotsReturnsNotFoundWhenSessionDoesNotExist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		listBattleSnapshotsFn: func(ctx context.Context, sessionID string, fromTick, toTick *int64) ([]services.BattleSnapshotView, error) {
			return nil, gorm.ErrRecordNotFound
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.GET("/battle/sessions/:sessionId/snapshots", handler.ListBattleSnapshots)

	req := httptest.NewRequest(http.MethodGet, "/battle/sessions/missing/snapshots", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "battle session does not exist" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestGetBattleReportReturnsNotFoundWhenSessionDoesNotExist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		getBattleReportFn: func(ctx context.Context, sessionID string) (services.BattleReport, error) {
			return services.BattleReport{}, gorm.ErrRecordNotFound
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.GET("/battle/sessions/:sessionId/report", handler.GetBattleReport)

	req := httptest.NewRequest(http.MethodGet, "/battle/sessions/missing/report", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "battle session does not exist" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestStopBattleSessionReturnsNotFoundWhenSessionDoesNotExist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		stopBattleSessionFn: func(ctx context.Context, sessionID string) (services.BattleStateSnapshot, error) {
			return services.BattleStateSnapshot{}, gorm.ErrRecordNotFound
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.POST("/battle/sessions/:sessionId/stop", handler.StopBattleSession)

	req := httptest.NewRequest(http.MethodPost, "/battle/sessions/missing/stop", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "battle session does not exist" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestReceiveRadarReportReturnsNotFoundWhenSessionDoesNotExist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		receiveRadarReportFn: func(ctx context.Context, report services.RadarReportPayload) (services.BattleStateSnapshot, error) {
			return services.BattleStateSnapshot{}, gorm.ErrRecordNotFound
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.POST("/radar/reports", handler.ReceiveRadarReport)

	req := httptest.NewRequest(http.MethodPost, "/radar/reports", bytes.NewBufferString(`{"sessionId":"missing","radarId":"radar-1","targets":[]}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "battle session does not exist" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestReceiveRadarReportReturnsValidationMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		receiveRadarReportFn: func(ctx context.Context, report services.RadarReportPayload) (services.BattleStateSnapshot, error) {
			return services.BattleStateSnapshot{}, services.NewValidationError("sessionId and radarId are required")
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.POST("/radar/reports", handler.ReceiveRadarReport)

	req := httptest.NewRequest(http.MethodPost, "/radar/reports", bytes.NewBufferString(`{"sessionId":"battle-1","radarId":"radar-1","targets":[]}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "sessionId and radarId are required" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestReceiveRadarReportReturnsInternalServerErrorWithoutLeakingStorageError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		receiveRadarReportFn: func(ctx context.Context, report services.RadarReportPayload) (services.BattleStateSnapshot, error) {
			return services.BattleStateSnapshot{}, io.ErrUnexpectedEOF
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.POST("/radar/reports", handler.ReceiveRadarReport)

	req := httptest.NewRequest(http.MethodPost, "/radar/reports", bytes.NewBufferString(`{"sessionId":"battle-1","radarId":"radar-1","targets":[]}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "failed to receive radar report" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestUpdateDispatchStatusReturnsInternalServerErrorWithoutLeakingStorageError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, &fakeAppService{
		updateDispatchStatusFn: func(ctx context.Context, id uint, toStatus, remark string, operatorID *uint) (models.DispatchEvent, error) {
			return models.DispatchEvent{}, io.ErrUnexpectedEOF
		},
	}, nil, nil, 0, false, nil)
	router := gin.New()
	router.PUT("/dispatch-events/:id/status", handler.UpdateDispatchStatus)

	req := httptest.NewRequest(http.MethodPut, "/dispatch-events/9/status", bytes.NewBufferString(`{"status":"PROCESSING"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "failed to update dispatch status" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestAnalyticsStatusReturnsBadGatewayWithoutLeakingInternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, nil, &fakeAnalyticsService{
		statusFn: func(ctx context.Context) (map[string]interface{}, error) {
			return nil, io.ErrUnexpectedEOF
		},
	}, nil, 0, false, nil)
	router := gin.New()
	router.GET("/analytics/simulate/status", handler.AnalyticsStatus)

	req := httptest.NewRequest(http.MethodGet, "/analytics/simulate/status", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "analytics service request failed" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestAnalyticsStatusReturnsStructuredUpstreamError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, nil, &fakeAnalyticsService{
		statusFn: func(ctx context.Context) (map[string]interface{}, error) {
			return nil, &services.AnalyticsError{
				StatusCode: http.StatusServiceUnavailable,
				Message:    "upstream unavailable",
				RequestID:  "trace-upstream",
			}
		},
	}, nil, 0, false, nil)
	router := gin.New()
	router.GET("/analytics/simulate/status", handler.AnalyticsStatus)

	req := httptest.NewRequest(http.MethodGet, "/analytics/simulate/status", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "upstream unavailable" {
		t.Fatalf("unexpected body: %#v", body)
	}
	if body["upstreamStatus"] != float64(http.StatusServiceUnavailable) {
		t.Fatalf("unexpected upstreamStatus: %#v", body)
	}
	if body["upstreamRequestId"] != "trace-upstream" {
		t.Fatalf("unexpected upstreamRequestId: %#v", body)
	}
}

func TestStartSimulationReturnsValidationMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(
		nil,
		nil,
		services.NewAnalyticsService(config.Config{AnalyticsBaseURL: "http://127.0.0.1:65535"}),
		nil,
		0,
		false,
		nil,
	)
	router := gin.New()
	router.POST("/analytics/simulate/start", handler.StartSimulation)

	req := httptest.NewRequest(http.MethodPost, "/analytics/simulate/start", bytes.NewBufferString(`{"shipIds":[0]}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "shipIds must contain only positive IDs" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestStartSimulationReturnsStructuredUpstreamError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, nil, &fakeAnalyticsService{
		startSimulationFn: func(ctx context.Context, req services.SimulationStartRequest) (map[string]interface{}, error) {
			return nil, &services.AnalyticsError{
				StatusCode: http.StatusTooManyRequests,
				Message:    "rate limited by analytics",
				RequestID:  "trace-analytics-start",
			}
		},
	}, nil, 0, false, nil)
	router := gin.New()
	router.POST("/analytics/simulate/start", handler.StartSimulation)

	req := httptest.NewRequest(http.MethodPost, "/analytics/simulate/start", bytes.NewBufferString(`{"shipIds":[1,2]}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "rate limited by analytics" {
		t.Fatalf("unexpected body: %#v", body)
	}
	if body["upstreamStatus"] != float64(http.StatusTooManyRequests) {
		t.Fatalf("unexpected upstreamStatus: %#v", body)
	}
	if body["upstreamRequestId"] != "trace-analytics-start" {
		t.Fatalf("unexpected upstreamRequestId: %#v", body)
	}
}

func TestStopSimulationReturnsBadGatewayWithoutLeakingInternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, nil, &fakeAnalyticsService{
		stopSimulationFn: func(ctx context.Context) (map[string]interface{}, error) {
			return nil, io.ErrUnexpectedEOF
		},
	}, nil, 0, false, nil)
	router := gin.New()
	router.POST("/analytics/simulate/stop", handler.StopSimulation)

	req := httptest.NewRequest(http.MethodPost, "/analytics/simulate/stop", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "analytics service request failed" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestStartBattleSimulationReturnsValidationMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(
		nil,
		nil,
		services.NewAnalyticsService(config.Config{AnalyticsBaseURL: "http://127.0.0.1:65535"}),
		nil,
		0,
		false,
		nil,
	)
	router := gin.New()
	router.POST("/analytics/simulate/battle/start", handler.StartBattleSimulation)

	req := httptest.NewRequest(http.MethodPost, "/analytics/simulate/battle/start", bytes.NewBufferString(`{"sessionId":"battle-1","scenarioCode":"open-water-duel","originLongitude":999,"originLatitude":30}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "battle simulation origin coordinates are invalid" {
		t.Fatalf("expected validation message, got %#v", body)
	}
}

func TestStartBattleSimulationReturnsStructuredUpstreamError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, nil, &fakeAnalyticsService{
		startBattleSimulationFn: func(ctx context.Context, req services.BattleSimulationStartRequest) (map[string]interface{}, error) {
			return nil, &services.AnalyticsError{
				StatusCode: http.StatusServiceUnavailable,
				Message:    "battle simulator unavailable",
				RequestID:  "trace-battle-start",
			}
		},
	}, nil, 0, false, nil)
	router := gin.New()
	router.POST("/analytics/simulate/battle/start", handler.StartBattleSimulation)

	req := httptest.NewRequest(http.MethodPost, "/analytics/simulate/battle/start", bytes.NewBufferString(`{"sessionId":"battle-1","scenarioCode":"open-water-duel","originLongitude":121.49,"originLatitude":31.23}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "battle simulator unavailable" {
		t.Fatalf("unexpected body: %#v", body)
	}
	if body["upstreamStatus"] != float64(http.StatusServiceUnavailable) {
		t.Fatalf("unexpected upstreamStatus: %#v", body)
	}
	if body["upstreamRequestId"] != "trace-battle-start" {
		t.Fatalf("unexpected upstreamRequestId: %#v", body)
	}
}

func TestStopBattleSimulationReturnsValidationMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(
		nil,
		nil,
		services.NewAnalyticsService(config.Config{AnalyticsBaseURL: "http://127.0.0.1:65535"}),
		nil,
		0,
		false,
		nil,
	)
	router := gin.New()
	router.POST("/analytics/simulate/battle/stop", handler.StopBattleSimulation)

	req := httptest.NewRequest(http.MethodPost, "/analytics/simulate/battle/stop", bytes.NewBufferString(`{"sessionId":"   "}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "sessionId is required" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestStopBattleSimulationReturnsBadGatewayWithoutLeakingInternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, nil, &fakeAnalyticsService{
		stopBattleSimulationFn: func(ctx context.Context, req services.BattleSimulationStopRequest) (map[string]interface{}, error) {
			return nil, io.ErrUnexpectedEOF
		},
	}, nil, 0, false, nil)
	router := gin.New()
	router.POST("/analytics/simulate/battle/stop", handler.StopBattleSimulation)

	req := httptest.NewRequest(http.MethodPost, "/analytics/simulate/battle/stop", bytes.NewBufferString(`{"sessionId":"battle-1"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "analytics service request failed" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestMonitorWSReturnsUnauthorizedWhenAuthTokenIsMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(&fakeAuthService{}, nil, nil, nil, 0, false, nil)
	router := gin.New()
	router.GET("/ws/monitor", handler.MonitorWS)

	req := httptest.NewRequest(http.MethodGet, "/ws/monitor", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "missing auth token" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestMonitorWSReturnsUnauthorizedWhenAuthTokenIsInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(&fakeAuthService{
		parseTokenFn: func(tokenText string) (*services.Claims, error) {
			return nil, io.ErrUnexpectedEOF
		},
	}, nil, nil, nil, 0, false, nil)
	router := gin.New()
	router.GET("/ws/monitor", handler.MonitorWS)

	req := httptest.NewRequest(http.MethodGet, "/ws/monitor?token=bad-token", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
	body := decodeJSONBody(t, recorder)
	if body["message"] != "invalid auth token" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func decodeJSONBody(t *testing.T, recorder *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	body, err := io.ReadAll(recorder.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode body %q: %v", string(body), err)
	}
	return payload
}
