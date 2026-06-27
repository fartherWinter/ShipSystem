package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"shipsystem/backend/internal/middleware"
	"shipsystem/backend/internal/models"
	"shipsystem/backend/internal/services"
	"shipsystem/backend/internal/ws"
)

type Handler struct {
	auth           authService
	app            appService
	analytics      analyticsService
	hub            *ws.Hub
	cookieMaxAge   int
	secureCookies  bool
	allowedOrigins map[string]struct{}
	allowAnyOrigin bool
}

type appService interface {
	ListShips(ctx context.Context, keyword string, page, size int) ([]models.Ship, int64, error)
	CreateShip(ctx context.Context, ship *models.Ship) error
	GetShip(ctx context.Context, id uint) (models.Ship, error)
	UpdateShip(ctx context.Context, id uint, patch models.Ship) (models.Ship, error)
	DeleteShip(ctx context.Context, id uint) error
	ReportLocation(ctx context.Context, loc *models.ShipLocation) (*models.Alarm, error)
	ListTracks(ctx context.Context, shipID uint, start, end *time.Time) ([]models.ShipLocation, error)
	ListAlarms(ctx context.Context, status string, page, size int) ([]models.Alarm, int64, error)
	AckAlarm(ctx context.Context, id, userID uint) (models.Alarm, error)
	ListDispatchEvents(ctx context.Context, status string, page, size int) ([]models.DispatchEvent, int64, error)
	CreateDispatchEvent(ctx context.Context, event *models.DispatchEvent) error
	UpdateDispatchStatus(ctx context.Context, id uint, toStatus, remark string, operatorID *uint) (models.DispatchEvent, error)
	ListUsers(ctx context.Context) ([]models.User, error)
	ListRoles(ctx context.Context) ([]models.Role, error)
	ListMenus(ctx context.Context) ([]models.Menu, error)
	ListBattleScenarios(ctx context.Context) []services.BattleScenario
	ListBattleSessions(ctx context.Context, page, size int) ([]models.BattleSession, int64, error)
	CreateBattleSession(ctx context.Context, scenarioCode string) (models.BattleSession, services.BattleStateSnapshot, error)
	GetBattleState(ctx context.Context, sessionID string) (services.BattleStateSnapshot, error)
	ListBattleTimeline(ctx context.Context, sessionID string) ([]services.BattleTimelineItem, error)
	ListBattleSnapshots(ctx context.Context, sessionID string, fromTick, toTick *int64) ([]services.BattleSnapshotView, error)
	GetBattleReport(ctx context.Context, sessionID string) (services.BattleReport, error)
	StopBattleSession(ctx context.Context, sessionID string) (services.BattleStateSnapshot, error)
	ReceiveRadarReport(ctx context.Context, report services.RadarReportPayload) (services.BattleStateSnapshot, error)
}

type analyticsService interface {
	Status(ctx context.Context) (map[string]interface{}, error)
	StartSimulation(ctx context.Context, req services.SimulationStartRequest) (map[string]interface{}, error)
	StopSimulation(ctx context.Context) (map[string]interface{}, error)
	StartBattleSimulation(ctx context.Context, req services.BattleSimulationStartRequest) (map[string]interface{}, error)
	StopBattleSimulation(ctx context.Context, req services.BattleSimulationStopRequest) (map[string]interface{}, error)
}

type authService interface {
	Login(ctx context.Context, username, password string) (string, models.User, []models.Menu, error)
	ParseToken(tokenText string) (*services.Claims, error)
}

var _ authService = (*services.AuthService)(nil)
var _ appService = (*services.AppService)(nil)
var _ analyticsService = (*services.AnalyticsService)(nil)

func NewHandler(auth authService, app appService, analytics analyticsService, hub *ws.Hub, cookieMaxAge int, secureCookies bool, allowedOrigins []string) *Handler {
	origins := make(map[string]struct{}, len(allowedOrigins))
	allowAnyOrigin := false
	for _, origin := range allowedOrigins {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		if origin == "*" {
			allowAnyOrigin = true
			continue
		}
		origins[origin] = struct{}{}
	}
	return &Handler{
		auth:           auth,
		app:            app,
		analytics:      analytics,
		hub:            hub,
		cookieMaxAge:   cookieMaxAge,
		secureCookies:  secureCookies,
		allowedOrigins: origins,
		allowAnyOrigin: allowAnyOrigin,
	}
}

func (h *Handler) RegisterRoutes(r gin.IRouter) {
	shipReaders := middleware.RequireRoles(middleware.RoleAdmin, middleware.RoleDispatcher, middleware.RoleViewer)
	shipWriters := middleware.RequireRoles(middleware.RoleAdmin)
	dispatchers := middleware.RequireRoles(middleware.RoleAdmin, middleware.RoleDispatcher)
	dispatchersAndAnalytics := middleware.RequireRoles(middleware.RoleAdmin, middleware.RoleDispatcher, middleware.RoleAnalytics)
	viewers := middleware.RequireRoles(middleware.RoleAdmin, middleware.RoleDispatcher, middleware.RoleViewer)
	superAdmins := middleware.RequireRoles()

	r.GET("/ships", shipReaders, h.ListShips)
	r.POST("/ships", shipWriters, h.CreateShip)
	r.GET("/ships/:id", shipReaders, h.GetShip)
	r.PUT("/ships/:id", shipWriters, h.UpdateShip)
	r.DELETE("/ships/:id", shipWriters, h.DeleteShip)
	r.POST("/ships/:id/locations", dispatchersAndAnalytics, h.ReportLocation)
	r.GET("/ships/:id/tracks", shipReaders, h.ListTracks)

	r.GET("/alarms", viewers, h.ListAlarms)
	r.PUT("/alarms/:id/ack", dispatchers, h.AckAlarm)

	r.GET("/dispatch-events", viewers, h.ListDispatchEvents)
	r.POST("/dispatch-events", dispatchers, h.CreateDispatchEvent)
	r.PUT("/dispatch-events/:id/status", dispatchers, h.UpdateDispatchStatus)

	r.GET("/battle/scenarios", viewers, h.ListBattleScenarios)
	r.GET("/battle/sessions", viewers, h.ListBattleSessions)
	r.POST("/battle/sessions", dispatchers, h.CreateBattleSession)
	r.GET("/battle/sessions/:sessionId/timeline", viewers, h.GetBattleTimeline)
	r.GET("/battle/sessions/:sessionId/snapshots", viewers, h.ListBattleSnapshots)
	r.GET("/battle/sessions/:sessionId/report", viewers, h.GetBattleReport)
	r.GET("/battle/sessions/:sessionId/state", viewers, h.GetBattleState)
	r.POST("/battle/sessions/:sessionId/stop", dispatchers, h.StopBattleSession)
	r.POST("/radar/reports", dispatchersAndAnalytics, h.ReceiveRadarReport)

	r.GET("/analytics/simulate/status", dispatchers, h.AnalyticsStatus)
	r.POST("/analytics/simulate/start", dispatchers, h.StartSimulation)
	r.POST("/analytics/simulate/stop", dispatchers, h.StopSimulation)
	r.POST("/analytics/simulate/battle/start", dispatchers, h.StartBattleSimulation)
	r.POST("/analytics/simulate/battle/stop", dispatchers, h.StopBattleSimulation)

	r.GET("/rbac/users", superAdmins, h.ListUsers)
	r.GET("/rbac/roles", superAdmins, h.ListRoles)
	r.GET("/rbac/menus", superAdmins, h.ListMenus)
}

func (h *Handler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "request validation failed")
		return
	}
	token, user, menus, err := h.auth.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		status := http.StatusInternalServerError
		message := "failed to complete login"
		if services.IsAuthenticationError(err) {
			status = http.StatusUnauthorized
			message = err.Error()
		}
		respondError(c, status, message)
		return
	}
	setAuthCookie(c, token, h.cookieMaxAge, h.secureCookies)
	c.JSON(http.StatusOK, gin.H{"token": token, "user": user, "menus": menus})
}

func (h *Handler) Logout(c *gin.Context) {
	clearAuthCookie(c, h.secureCookies)
	c.Status(http.StatusNoContent)
}

func (h *Handler) ListShips(c *gin.Context) {
	page, size, ok := pagination(c)
	if !ok {
		return
	}
	ships, total, err := h.app.ListShips(c.Request.Context(), c.Query("keyword"), page, size)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to list ships")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": ships, "total": total})
}

func (h *Handler) CreateShip(c *gin.Context) {
	var req models.Ship
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" || req.MMSI == "" {
		respondError(c, http.StatusBadRequest, "ship name and MMSI are required")
		return
	}
	if err := h.app.CreateShip(c.Request.Context(), &req); err != nil {
		status := http.StatusInternalServerError
		message := "failed to create ship"
		if services.IsValidationError(err) {
			status = http.StatusBadRequest
			message = err.Error()
		}
		respondError(c, status, message)
		return
	}
	c.JSON(http.StatusCreated, req)
}

func (h *Handler) GetShip(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		respondError(c, http.StatusBadRequest, "ship id must be a positive integer")
		return
	}
	ship, err := h.app.GetShip(c.Request.Context(), id)
	if err != nil {
		status := statusForAppError(err)
		message := "failed to get ship"
		if status == http.StatusNotFound {
			message = "ship does not exist"
		}
		respondError(c, status, message)
		return
	}
	c.JSON(http.StatusOK, ship)
}

func (h *Handler) UpdateShip(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		respondError(c, http.StatusBadRequest, "ship id must be a positive integer")
		return
	}
	var req models.Ship
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" || req.MMSI == "" {
		respondError(c, http.StatusBadRequest, "ship name and MMSI are required")
		return
	}
	ship, err := h.app.UpdateShip(c.Request.Context(), id, req)
	if err != nil {
		status := statusForAppError(err)
		message := "failed to update ship"
		if status == http.StatusNotFound {
			message = "ship does not exist"
		}
		if status == http.StatusBadRequest {
			message = err.Error()
		}
		respondError(c, status, message)
		return
	}
	c.JSON(http.StatusOK, ship)
}

func (h *Handler) DeleteShip(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		respondError(c, http.StatusBadRequest, "ship id must be a positive integer")
		return
	}
	if err := h.app.DeleteShip(c.Request.Context(), id); err != nil {
		status := statusForAppError(err)
		message := "failed to delete ship"
		if status == http.StatusNotFound {
			message = "ship does not exist"
		}
		respondError(c, status, message)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ReportLocation(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		respondError(c, http.StatusBadRequest, "ship id must be a positive integer")
		return
	}
	var req struct {
		Longitude  float64    `json:"longitude" binding:"required"`
		Latitude   float64    `json:"latitude" binding:"required"`
		SpeedKnots float64    `json:"speedKnots"`
		Course     float64    `json:"course"`
		ReportedAt *time.Time `json:"reportedAt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "location report request is invalid")
		return
	}
	if !validLongitude(req.Longitude) || !validLatitude(req.Latitude) || req.SpeedKnots < 0 || req.Course < 0 || req.Course >= 360 {
		respondError(c, http.StatusBadRequest, "location report fields are invalid")
		return
	}
	reportedAt := time.Now()
	if req.ReportedAt != nil {
		reportedAt = *req.ReportedAt
	}
	loc := models.ShipLocation{
		ShipID:     id,
		Longitude:  req.Longitude,
		Latitude:   req.Latitude,
		SpeedKnots: req.SpeedKnots,
		Course:     req.Course,
		ReportedAt: reportedAt,
	}
	alarm, err := h.app.ReportLocation(c.Request.Context(), &loc)
	if err != nil {
		status := statusForAppError(err)
		message := "failed to report location"
		if status == http.StatusNotFound {
			message = "ship does not exist"
		}
		respondError(c, status, message)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"location": loc, "alarm": alarm})
}

func (h *Handler) ListTracks(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		respondError(c, http.StatusBadRequest, "ship id must be a positive integer")
		return
	}
	start, err := parseTime(c.Query("start"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "start must be a valid RFC3339 timestamp")
		return
	}
	end, err := parseTime(c.Query("end"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "end must be a valid RFC3339 timestamp")
		return
	}
	tracks, err := h.app.ListTracks(c.Request.Context(), id, start, end)
	if err != nil {
		status := statusForAppError(err)
		message := "failed to list tracks"
		if status == http.StatusNotFound {
			message = "ship does not exist"
		}
		respondError(c, status, message)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": tracks})
}

func (h *Handler) ListAlarms(c *gin.Context) {
	page, size, ok := pagination(c)
	if !ok {
		return
	}
	alarms, total, err := h.app.ListAlarms(c.Request.Context(), c.Query("status"), page, size)
	if err != nil {
		status := statusForAppError(err)
		message := "failed to list alarms"
		if status == http.StatusBadRequest {
			message = err.Error()
		}
		respondError(c, status, message)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": alarms, "total": total})
}

func (h *Handler) AckAlarm(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		respondError(c, http.StatusBadRequest, "alarm id must be a positive integer")
		return
	}
	alarm, err := h.app.AckAlarm(c.Request.Context(), id, middleware.CurrentUserID(c))
	if err != nil {
		status := statusForAppError(err)
		message := "failed to ack alarm"
		if status == http.StatusNotFound {
			message = "alarm does not exist"
		}
		respondError(c, status, message)
		return
	}
	c.JSON(http.StatusOK, alarm)
}

func (h *Handler) ListDispatchEvents(c *gin.Context) {
	page, size, ok := pagination(c)
	if !ok {
		return
	}
	events, total, err := h.app.ListDispatchEvents(c.Request.Context(), c.Query("status"), page, size)
	if err != nil {
		status := statusForAppError(err)
		message := "failed to list dispatch events"
		if status == http.StatusBadRequest {
			message = err.Error()
		}
		respondError(c, status, message)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": events, "total": total})
}

func (h *Handler) CreateDispatchEvent(c *gin.Context) {
	var req models.DispatchEvent
	if err := c.ShouldBindJSON(&req); err != nil || req.Title == "" {
		respondError(c, http.StatusBadRequest, "dispatch event title is required")
		return
	}
	userID := middleware.CurrentUserID(c)
	req.CreatedByID = &userID
	if err := h.app.CreateDispatchEvent(c.Request.Context(), &req); err != nil {
		status := statusForAppError(err)
		message := "failed to create dispatch event"
		if status == http.StatusBadRequest {
			message = err.Error()
		}
		respondError(c, status, message)
		return
	}
	c.JSON(http.StatusCreated, req)
}

func (h *Handler) UpdateDispatchStatus(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		respondError(c, http.StatusBadRequest, "dispatch event id must be a positive integer")
		return
	}
	var req struct {
		Status string `json:"status" binding:"required"`
		Remark string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "dispatch status request is invalid")
		return
	}
	userID := middleware.CurrentUserID(c)
	event, err := h.app.UpdateDispatchStatus(c.Request.Context(), id, req.Status, req.Remark, &userID)
	if err != nil {
		status := statusForAppError(err)
		message := err.Error()
		if status == http.StatusNotFound {
			message = "dispatch event does not exist"
		} else if status == http.StatusInternalServerError {
			message = "failed to update dispatch status"
		}
		respondError(c, status, message)
		return
	}
	c.JSON(http.StatusOK, event)
}

func (h *Handler) ListBattleScenarios(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": h.app.ListBattleScenarios(c.Request.Context())})
}

func (h *Handler) ListBattleSessions(c *gin.Context) {
	page, size, ok := pagination(c)
	if !ok {
		return
	}
	sessions, total, err := h.app.ListBattleSessions(c.Request.Context(), page, size)
	if err != nil {
		respondError(c, statusForAppError(err), "failed to list battle sessions")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": sessions, "total": total})
}

func (h *Handler) CreateBattleSession(c *gin.Context) {
	var req struct {
		ScenarioCode string `json:"scenarioCode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		respondError(c, http.StatusBadRequest, "battle session request is invalid")
		return
	}
	session, state, err := h.app.CreateBattleSession(c.Request.Context(), req.ScenarioCode)
	if err != nil {
		status := statusForAppError(err)
		message := "failed to create battle session"
		if status == http.StatusBadRequest {
			message = err.Error()
		}
		respondError(c, status, message)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"session": session, "state": state})
}

func (h *Handler) GetBattleState(c *gin.Context) {
	state, err := h.app.GetBattleState(c.Request.Context(), c.Param("sessionId"))
	if err != nil {
		status := statusForAppError(err)
		message := "failed to load battle state"
		if status == http.StatusNotFound {
			message = "battle session does not exist"
		}
		respondError(c, status, message)
		return
	}
	c.JSON(http.StatusOK, state)
}

func (h *Handler) GetBattleTimeline(c *gin.Context) {
	items, err := h.app.ListBattleTimeline(c.Request.Context(), c.Param("sessionId"))
	if err != nil {
		status := statusForAppError(err)
		message := "failed to load battle timeline"
		if status == http.StatusNotFound {
			message = "battle session does not exist"
		}
		respondError(c, status, message)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) ListBattleSnapshots(c *gin.Context) {
	fromTick, err := tickQuery(c, "from")
	if err != nil {
		respondError(c, http.StatusBadRequest, "from must be a valid tick")
		return
	}
	toTick, err := tickQuery(c, "to")
	if err != nil {
		respondError(c, http.StatusBadRequest, "to must be a valid tick")
		return
	}
	if fromTick != nil && toTick != nil && *fromTick > *toTick {
		respondError(c, http.StatusBadRequest, "from must be less than or equal to to")
		return
	}
	items, err := h.app.ListBattleSnapshots(c.Request.Context(), c.Param("sessionId"), fromTick, toTick)
	if err != nil {
		status := statusForAppError(err)
		message := "failed to load battle snapshots"
		if status == http.StatusNotFound {
			message = "battle session does not exist"
		}
		respondError(c, status, message)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) GetBattleReport(c *gin.Context) {
	report, err := h.app.GetBattleReport(c.Request.Context(), c.Param("sessionId"))
	if err != nil {
		status := statusForAppError(err)
		message := "failed to load battle report"
		if status == http.StatusNotFound {
			message = "battle session does not exist"
		}
		respondError(c, status, message)
		return
	}
	c.JSON(http.StatusOK, report)
}

func (h *Handler) StopBattleSession(c *gin.Context) {
	state, err := h.app.StopBattleSession(c.Request.Context(), c.Param("sessionId"))
	if err != nil {
		status := statusForAppError(err)
		message := "failed to stop battle session"
		if status == http.StatusNotFound {
			message = "battle session does not exist"
		}
		respondError(c, status, message)
		return
	}
	c.JSON(http.StatusOK, state)
}

func (h *Handler) ReceiveRadarReport(c *gin.Context) {
	var req services.RadarReportPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "radar report request is invalid")
		return
	}
	state, err := h.app.ReceiveRadarReport(c.Request.Context(), req)
	if err != nil {
		status := statusForAppError(err)
		message := err.Error()
		if status == http.StatusNotFound {
			message = "battle session does not exist"
		} else if status == http.StatusInternalServerError {
			message = "failed to receive radar report"
		}
		respondError(c, status, message)
		return
	}
	c.JSON(http.StatusAccepted, state)
}

func (h *Handler) AnalyticsStatus(c *gin.Context) {
	data, err := h.analytics.Status(c.Request.Context())
	respondAnalytics(c, data, err)
}

func (h *Handler) StartSimulation(c *gin.Context) {
	var req services.SimulationStartRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		respondError(c, http.StatusBadRequest, "simulation request is invalid")
		return
	}
	data, err := h.analytics.StartSimulation(c.Request.Context(), req)
	respondAnalytics(c, data, err)
}

func (h *Handler) StopSimulation(c *gin.Context) {
	data, err := h.analytics.StopSimulation(c.Request.Context())
	respondAnalytics(c, data, err)
}

func (h *Handler) StartBattleSimulation(c *gin.Context) {
	var req services.BattleSimulationStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "battle simulation request is invalid")
		return
	}
	data, err := h.analytics.StartBattleSimulation(c.Request.Context(), req)
	respondAnalytics(c, data, err)
}

func (h *Handler) StopBattleSimulation(c *gin.Context) {
	var req services.BattleSimulationStopRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "battle simulation stop request is invalid")
		return
	}
	data, err := h.analytics.StopBattleSimulation(c.Request.Context(), req)
	respondAnalytics(c, data, err)
}

func (h *Handler) ListUsers(c *gin.Context) {
	items, err := h.app.ListUsers(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to list users")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) ListRoles(c *gin.Context) {
	items, err := h.app.ListRoles(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to list roles")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) ListMenus(c *gin.Context) {
	items, err := h.app.ListMenus(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to list menus")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) MonitorWS(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		cookieToken, err := c.Cookie(middleware.TokenCookieName)
		if err != nil {
			respondError(c, http.StatusUnauthorized, "missing auth token")
			return
		}
		token = cookieToken
	}
	if _, err := h.auth.ParseToken(token); err != nil {
		respondError(c, http.StatusUnauthorized, "invalid auth token")
		return
	}
	upgrader := websocket.Upgrader{
		CheckOrigin: h.checkWSOrigin,
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	client := h.hub.Register(conn)
	go client.WritePump()
	client.ReadPump()
}

func (h *Handler) checkWSOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if h.allowAnyOrigin {
		return true
	}
	_, ok := h.allowedOrigins[origin]
	return ok
}

func respondError(c *gin.Context, status int, message string) {
	payload := gin.H{"message": message}
	if requestID := middleware.CurrentRequestID(c); requestID != "" {
		payload["requestId"] = requestID
	}
	c.JSON(status, payload)
}

func statusForAppError(err error) int {
	if services.IsNotFound(err) {
		return http.StatusNotFound
	}
	if services.IsValidationError(err) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func setAuthCookie(c *gin.Context, token string, maxAge int, forceSecure bool) {
	if maxAge <= 0 {
		maxAge = 24 * 60 * 60
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     middleware.TokenCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secureRequest(c) || forceSecure,
	})
}

func clearAuthCookie(c *gin.Context, forceSecure bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     middleware.TokenCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secureRequest(c) || forceSecure,
	})
}

func secureRequest(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}
	return strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
}

func respondAnalytics(c *gin.Context, data map[string]interface{}, err error) {
	if err != nil {
		status := http.StatusBadGateway
		message := "analytics service request failed"
		if services.IsValidationError(err) {
			status = http.StatusBadRequest
			message = err.Error()
		}
		if analyticsErr, ok := services.AsAnalyticsError(err); ok {
			payload := gin.H{
				"message":        analyticsErr.Message,
				"upstreamStatus": analyticsErr.StatusCode,
			}
			if payload["message"] == "" {
				payload["message"] = "analytics service request failed"
			}
			if requestID := middleware.CurrentRequestID(c); requestID != "" {
				payload["requestId"] = requestID
			}
			if analyticsErr.RequestID != "" {
				payload["upstreamRequestId"] = analyticsErr.RequestID
			}
			c.JSON(status, payload)
			return
		}
		respondError(c, status, message)
		return
	}
	c.JSON(http.StatusOK, data)
}

func idParam(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return 0, false
	}
	return uint(id), true
}

func pagination(c *gin.Context) (int, int, bool) {
	page, err := positiveIntQuery(c, "page", 1)
	if err != nil {
		respondError(c, http.StatusBadRequest, "page must be a positive integer")
		return 0, 0, false
	}
	size, err := positiveIntQuery(c, "size", 20)
	if err != nil {
		respondError(c, http.StatusBadRequest, "size must be a positive integer")
		return 0, 0, false
	}
	if size > 100 {
		respondError(c, http.StatusBadRequest, "size must be less than or equal to 100")
		return 0, 0, false
	}
	return page, size, true
}

func positiveIntQuery(c *gin.Context, name string, defaultValue int) (int, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, strconv.ErrSyntax
	}
	return value, nil
}

func tickQuery(c *gin.Context, name string) (*int64, error) {
	value := strings.TrimSpace(c.Query(name))
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return nil, strconv.ErrSyntax
	}
	return &parsed, nil
}

func parseTime(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func validLongitude(value float64) bool {
	return value >= -180 && value <= 180
}

func validLatitude(value float64) bool {
	return value >= -90 && value <= 90
}
