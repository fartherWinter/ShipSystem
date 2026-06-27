package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"shipsystem/backend/internal/config"
	"shipsystem/backend/internal/models"
	"shipsystem/backend/internal/repositories"
	"shipsystem/backend/internal/ws"
)

type AuthService struct {
	store *repositories.Store
	cfg   config.Config
}

type Claims struct {
	UserID   uint   `json:"userId"`
	Username string `json:"username"`
	RoleCode string `json:"roleCode"`
	jwt.RegisteredClaims
}

func NewAuthService(store *repositories.Store, cfg config.Config) *AuthService {
	return &AuthService{store: store, cfg: cfg}
}

func (s *AuthService) Login(ctx context.Context, username, password string) (string, models.User, []models.Menu, error) {
	user, err := s.store.FindUserByUsername(ctx, username)
	if err != nil {
		return "", models.User{}, nil, NewAuthenticationError("用户名或密码错误")
	}
	if user.Status != "enabled" {
		return "", models.User{}, nil, NewAuthenticationError("用户已禁用")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", models.User{}, nil, NewAuthenticationError("用户名或密码错误")
	}
	menus, err := s.store.ListMenus(ctx)
	if err != nil {
		return "", models.User{}, nil, err
	}
	token, err := s.generateToken(user)
	return token, user, menus, err
}

func (s *AuthService) ParseToken(tokenText string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenText, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

func (s *AuthService) generateToken(user models.User) (string, error) {
	claims := Claims{
		UserID:   user.ID,
		Username: user.Username,
		RoleCode: user.Role.Code,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(uint64(user.ID), 10),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.cfg.JWTTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.JWTSecret))
}

type AppService struct {
	store *repositories.Store
	hub   *ws.Hub
}

type battleStore interface {
	GetBattleSession(ctx context.Context, sessionID string) (models.BattleSession, error)
	UpdateBattleSessionScan(ctx context.Context, sessionID, status string, scanTime time.Time) error
	UpsertRadarTargets(ctx context.Context, targets []models.RadarTarget) error
	UpsertBattleUnits(ctx context.Context, units []models.BattleUnit) error
	UpsertBattleProjectiles(ctx context.Context, projectiles []models.BattleProjectile) error
	CreateBattleEvents(ctx context.Context, events []models.BattleEvent) ([]models.BattleEvent, error)
	AppendBattleSnapshot(ctx context.Context, snapshot *models.BattleSnapshot) (bool, error)
	ListBattleUnits(ctx context.Context, sessionID string) ([]models.BattleUnit, error)
	ListBattleProjectiles(ctx context.Context, sessionID string) ([]models.BattleProjectile, error)
	ListBattleEvents(ctx context.Context, sessionID string, limit int) ([]models.BattleEvent, error)
	ListRadarTargets(ctx context.Context, sessionID string) ([]models.RadarTarget, error)
}

type eventBroadcaster interface {
	Broadcast(event ws.Event) bool
}

type ValidationError struct {
	message string
}

func (e ValidationError) Error() string {
	return e.message
}

func NewValidationError(message string) ValidationError {
	return ValidationError{message: message}
}

type AuthenticationError struct {
	message string
}

func (e AuthenticationError) Error() string {
	return e.message
}

func NewAuthenticationError(message string) AuthenticationError {
	return AuthenticationError{message: message}
}

type BattleScenario struct {
	Code            string  `json:"code"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	OriginLongitude float64 `json:"originLongitude"`
	OriginLatitude  float64 `json:"originLatitude"`
	BlueUnits       int     `json:"blueUnits"`
	RedUnits        int     `json:"redUnits"`
	RadarRangeKm    float64 `json:"radarRangeKm"`
	WeaponRangeKm   float64 `json:"weaponRangeKm"`
}

type BattleStateSnapshot struct {
	SessionID    string                    `json:"sessionId"`
	Status       string                    `json:"status"`
	Units        []models.BattleUnit       `json:"units"`
	Projectiles  []models.BattleProjectile `json:"projectiles"`
	Events       []models.BattleEvent      `json:"events"`
	RadarTargets []models.RadarTarget      `json:"radarTargets"`
	UpdatedAt    time.Time                 `json:"updatedAt"`
}

type BattleSnapshotView struct {
	ID           uint                      `json:"id"`
	SessionID    string                    `json:"sessionId"`
	Tick         int64                     `json:"tick"`
	SnapshotTime time.Time                 `json:"snapshotTime"`
	Units        []models.BattleUnit       `json:"units"`
	Projectiles  []models.BattleProjectile `json:"projectiles"`
	RadarTargets []models.RadarTarget      `json:"radarTargets"`
	Events       []models.BattleEvent      `json:"events"`
}

type BattleTimelineItem struct {
	Tick         int64                        `json:"tick"`
	SnapshotTime time.Time                    `json:"snapshotTime"`
	EventCount   int                          `json:"eventCount"`
	Events       []BattleTimelineEventSummary `json:"events"`
}

type BattleTimelineEventSummary struct {
	Type         string `json:"type"`
	Severity     string `json:"severity"`
	Message      string `json:"message"`
	SourceUnitID string `json:"sourceUnitId"`
	TargetUnitID string `json:"targetUnitId"`
}

type BattleDamageStat struct {
	UnitID      string  `json:"unitId"`
	Name        string  `json:"name"`
	Side        string  `json:"side"`
	MaxHP       float64 `json:"maxHp"`
	HP          float64 `json:"hp"`
	DamageTaken float64 `json:"damageTaken"`
	Status      string  `json:"status"`
}

type BattleReport struct {
	Session        models.BattleSession `json:"session"`
	Winner         string               `json:"winner"`
	FiredCount     int                  `json:"firedCount"`
	HitCount       int                  `json:"hitCount"`
	DestroyedCount int                  `json:"destroyedCount"`
	DamageRanking  []BattleDamageStat   `json:"damageRanking"`
	KeyEvents      []models.BattleEvent `json:"keyEvents"`
}

type RadarReportPayload struct {
	SessionID string               `json:"sessionId"`
	RadarID   string               `json:"radarId"`
	ScanTime  *time.Time           `json:"scanTime"`
	Targets   []RadarTargetPayload `json:"targets"`
	State     *BattleStatePayload  `json:"state"`
}

type RadarTargetPayload struct {
	TargetID   string  `json:"targetId"`
	Side       string  `json:"side"`
	Longitude  float64 `json:"longitude"`
	Latitude   float64 `json:"latitude"`
	Course     float64 `json:"course"`
	SpeedKnots float64 `json:"speedKnots"`
	Confidence float64 `json:"confidence"`
	Detected   bool    `json:"detected"`
}

type BattleStatePayload struct {
	SessionID   string                    `json:"sessionId"`
	Status      string                    `json:"status"`
	Units       []BattleUnitPayload       `json:"units"`
	Projectiles []BattleProjectilePayload `json:"projectiles"`
	Events      []BattleEventPayload      `json:"events"`
	UpdatedAt   *time.Time                `json:"updatedAt"`
}

type BattleUnitPayload struct {
	UnitID          string  `json:"unitId"`
	ShipID          uint    `json:"shipId"`
	Name            string  `json:"name"`
	Side            string  `json:"side"`
	HP              float64 `json:"hp"`
	MaxHP           float64 `json:"maxHp"`
	RadarRangeKm    float64 `json:"radarRangeKm"`
	WeaponRangeKm   float64 `json:"weaponRangeKm"`
	CooldownSeconds float64 `json:"cooldownSeconds"`
	Longitude       float64 `json:"longitude"`
	Latitude        float64 `json:"latitude"`
	Course          float64 `json:"course"`
	SpeedKnots      float64 `json:"speedKnots"`
	Status          string  `json:"status"`
}

type BattleProjectilePayload struct {
	ProjectileID string  `json:"projectileId"`
	SourceUnitID string  `json:"sourceUnitId"`
	TargetUnitID string  `json:"targetUnitId"`
	Side         string  `json:"side"`
	Longitude    float64 `json:"longitude"`
	Latitude     float64 `json:"latitude"`
	SpeedKmH     float64 `json:"speedKmH"`
	Status       string  `json:"status"`
}

type BattleEventPayload struct {
	EventID      string     `json:"eventId"`
	Type         string     `json:"type"`
	Severity     string     `json:"severity"`
	Message      string     `json:"message"`
	SourceUnitID string     `json:"sourceUnitId"`
	TargetUnitID string     `json:"targetUnitId"`
	Longitude    *float64   `json:"longitude"`
	Latitude     *float64   `json:"latitude"`
	OccurredAt   *time.Time `json:"occurredAt"`
}

const (
	maxRadarTargetsPerReport      = 500
	maxBattleUnitsPerReport       = 200
	maxBattleProjectilesPerReport = 500
	maxBattleEventsPerReport      = 500
)

func NewAppService(store *repositories.Store, hub *ws.Hub) *AppService {
	return &AppService{store: store, hub: hub}
}

func (s *AppService) ListShips(ctx context.Context, keyword string, page, size int) ([]models.Ship, int64, error) {
	return s.store.ListShips(ctx, keyword, normalizePage(page), normalizeSize(size))
}

func (s *AppService) CreateShip(ctx context.Context, ship *models.Ship) error {
	if err := validateShip(ship); err != nil {
		return err
	}
	return s.store.CreateShip(ctx, ship)
}

func (s *AppService) GetShip(ctx context.Context, id uint) (models.Ship, error) {
	return s.store.GetShip(ctx, id)
}

func (s *AppService) UpdateShip(ctx context.Context, id uint, patch models.Ship) (models.Ship, error) {
	if err := validateShip(&patch); err != nil {
		return models.Ship{}, err
	}
	ship, err := s.store.GetShip(ctx, id)
	if err != nil {
		return models.Ship{}, err
	}
	ship.Name = patch.Name
	ship.MMSI = patch.MMSI
	ship.ShipType = patch.ShipType
	ship.Flag = patch.Flag
	ship.LengthM = patch.LengthM
	ship.WidthM = patch.WidthM
	ship.Status = patch.Status
	return ship, s.store.UpdateShip(ctx, &ship)
}

func (s *AppService) DeleteShip(ctx context.Context, id uint) error {
	return s.store.DeleteShip(ctx, id)
}

func (s *AppService) ReportLocation(ctx context.Context, loc *models.ShipLocation) (*models.Alarm, error) {
	if loc.ReportedAt.IsZero() {
		loc.ReportedAt = time.Now()
	}
	if _, err := s.store.GetShip(ctx, loc.ShipID); err != nil {
		return nil, err
	}
	if err := s.store.CreateLocation(ctx, loc); err != nil {
		return nil, err
	}
	payload := ws.Event{Type: "ship_location_updated", Data: loc}
	s.hub.Broadcast(payload)

	alarm, err := s.maybeCreateSpeedAlarm(ctx, loc)
	if err != nil {
		return nil, err
	}
	if alarm != nil {
		s.hub.Broadcast(ws.Event{Type: "alarm_created", Data: alarm})
	}
	return alarm, nil
}

func (s *AppService) maybeCreateSpeedAlarm(ctx context.Context, loc *models.ShipLocation) (*models.Alarm, error) {
	if loc.SpeedKnots <= 18 {
		return nil, nil
	}
	title := "船舶超速告警"
	alarm := &models.Alarm{
		ShipID:    loc.ShipID,
		Type:      "OVERSPEED",
		Level:     "WARN",
		Title:     title,
		Message:   fmt.Sprintf("当前航速 %.1f 节，超过 18 节阈值", loc.SpeedKnots),
		Longitude: &loc.Longitude,
		Latitude:  &loc.Latitude,
		Status:    "OPEN",
	}
	if err := s.store.CreateAlarm(ctx, alarm); err != nil {
		return nil, err
	}
	return alarm, nil
}

func (s *AppService) ListTracks(ctx context.Context, shipID uint, start, end *time.Time) ([]models.ShipLocation, error) {
	if _, err := s.store.GetShip(ctx, shipID); err != nil {
		return nil, err
	}
	return s.store.ListTracks(ctx, shipID, start, end)
}

func (s *AppService) ListAlarms(ctx context.Context, status string, page, size int) ([]models.Alarm, int64, error) {
	status = strings.TrimSpace(status)
	if status != "" && !isAlarmStatus(status) {
		return nil, 0, &ValidationError{message: "告警状态不合法"}
	}
	return s.store.ListAlarms(ctx, status, normalizePage(page), normalizeSize(size))
}

func (s *AppService) AckAlarm(ctx context.Context, id, userID uint) (models.Alarm, error) {
	alarm, changed, err := s.store.AckAlarm(ctx, id, userID)
	if err == nil && changed {
		s.hub.Broadcast(ws.Event{Type: "alarm_created", Data: alarm})
	}
	return alarm, err
}

func (s *AppService) ListDispatchEvents(ctx context.Context, status string, page, size int) ([]models.DispatchEvent, int64, error) {
	status = strings.TrimSpace(status)
	if status != "" && !isDispatchStatus(status) {
		return nil, 0, &ValidationError{message: "调度事件状态不合法"}
	}
	return s.store.ListDispatchEvents(ctx, status, normalizePage(page), normalizeSize(size))
}

func (s *AppService) CreateDispatchEvent(ctx context.Context, event *models.DispatchEvent) error {
	event.Title = strings.TrimSpace(event.Title)
	event.Description = strings.TrimSpace(event.Description)
	event.Status = strings.TrimSpace(event.Status)
	event.Priority = strings.TrimSpace(event.Priority)
	if event.Title == "" {
		return &ValidationError{message: "调度事件标题必填"}
	}
	if event.Status == "" {
		event.Status = "NEW"
	}
	if !isDispatchStatus(event.Status) {
		return &ValidationError{message: "调度事件状态不合法"}
	}
	if event.Priority == "" {
		event.Priority = "normal"
	}
	if !isDispatchPriority(event.Priority) {
		return &ValidationError{message: "调度事件优先级不合法"}
	}
	if err := s.store.CreateDispatchEvent(ctx, event); err != nil {
		return err
	}
	s.hub.Broadcast(ws.Event{Type: "dispatch_event_updated", Data: event})
	return nil
}

func (s *AppService) UpdateDispatchStatus(ctx context.Context, id uint, toStatus, remark string, operatorID *uint) (models.DispatchEvent, error) {
	event, err := s.store.GetDispatchEvent(ctx, id)
	if err != nil {
		return models.DispatchEvent{}, err
	}
	toStatus, remark, changed := normalizeDispatchStatusUpdate(event.Status, toStatus, remark)
	if !canTransitionDispatch(event.Status, toStatus) {
		return models.DispatchEvent{}, &ValidationError{message: fmt.Sprintf("调度事件不能从 %s 流转到 %s", event.Status, toStatus)}
	}
	if !changed {
		return event, nil
	}
	event, changed, err = s.store.UpdateDispatchStatus(ctx, id, toStatus, remark, operatorID)
	if err != nil {
		return models.DispatchEvent{}, err
	}
	if changed {
		s.hub.Broadcast(ws.Event{Type: "dispatch_event_updated", Data: event})
	}
	return event, nil
}

func (s *AppService) ListUsers(ctx context.Context) ([]models.User, error) {
	return s.store.ListUsers(ctx)
}

func (s *AppService) ListRoles(ctx context.Context) ([]models.Role, error) {
	return s.store.ListRoles(ctx)
}

func (s *AppService) ListMenus(ctx context.Context) ([]models.Menu, error) {
	return s.store.ListMenus(ctx)
}

func (s *AppService) ListBattleScenarios(ctx context.Context) []BattleScenario {
	return []BattleScenario{
		{
			Code:            "open-water-duel",
			Name:            "Open Water Radar Duel",
			Description:     "Two blue destroyers defend a patrol box against two red fast attack ships.",
			OriginLongitude: 121.49,
			OriginLatitude:  31.23,
			BlueUnits:       2,
			RedUnits:        2,
			RadarRangeKm:    32,
			WeaponRangeKm:   20,
		},
		{
			Code:            "close-quarter-barrage",
			Name:            "Close Quarter Barrage",
			Description:     "Shorter radar range and faster weapon tempo for dense projectile visualization.",
			OriginLongitude: 121.49,
			OriginLatitude:  31.23,
			BlueUnits:       2,
			RedUnits:        3,
			RadarRangeKm:    22,
			WeaponRangeKm:   15,
		},
	}
}

func (s *AppService) ListBattleSessions(ctx context.Context, page, size int) ([]models.BattleSession, int64, error) {
	return s.store.ListBattleSessions(ctx, normalizePage(page), normalizeSize(size))
}

func (s *AppService) CreateBattleSession(ctx context.Context, scenarioCode string) (models.BattleSession, BattleStateSnapshot, error) {
	scenario, err := s.findScenario(scenarioCode)
	if err != nil {
		return models.BattleSession{}, BattleStateSnapshot{}, err
	}
	now := time.Now()
	session := models.BattleSession{
		SessionID:    fmt.Sprintf("battle-%d", now.UnixNano()),
		Name:         scenario.Name,
		ScenarioCode: scenario.Code,
		Status:       "running",
		StartedAt:    now,
	}
	if err := s.store.CreateBattleSession(ctx, &session); err != nil {
		return models.BattleSession{}, BattleStateSnapshot{}, err
	}
	event := models.BattleEvent{
		SessionID:  session.SessionID,
		EventID:    fmt.Sprintf("%s-session-started", session.SessionID),
		Type:       "SESSION_STARTED",
		Severity:   "INFO",
		Message:    "Battle simulation session created.",
		OccurredAt: now,
	}
	_, _ = s.store.CreateBattleEvents(ctx, []models.BattleEvent{event})
	state, err := s.GetBattleState(ctx, session.SessionID)
	if err != nil {
		return models.BattleSession{}, BattleStateSnapshot{}, err
	}
	s.hub.Broadcast(ws.Event{Type: "battle_state_updated", Data: state})
	return session, state, nil
}

func (s *AppService) StopBattleSession(ctx context.Context, sessionID string) (BattleStateSnapshot, error) {
	if _, changed, err := s.store.StopBattleSession(ctx, sessionID); err != nil {
		return BattleStateSnapshot{}, err
	} else if !changed {
		return s.GetBattleState(ctx, sessionID)
	}
	now := time.Now()
	events := []models.BattleEvent{
		{
			SessionID:  sessionID,
			EventID:    fmt.Sprintf("%s-session-stopped-%d", sessionID, now.UnixNano()),
			Type:       "SESSION_STOPPED",
			Severity:   "INFO",
			Message:    "Battle simulation session stopped.",
			OccurredAt: now,
		},
	}
	_, _ = s.store.CreateBattleEvents(ctx, events)
	state, err := s.GetBattleState(ctx, sessionID)
	if err != nil {
		return BattleStateSnapshot{}, err
	}
	_, _ = s.saveBattleSnapshot(ctx, state, events)
	s.hub.Broadcast(ws.Event{Type: "battle_state_updated", Data: state})
	return state, nil
}

func (s *AppService) GetBattleState(ctx context.Context, sessionID string) (BattleStateSnapshot, error) {
	return getBattleStateFromStore(ctx, s.store, sessionID)
}

func getBattleStateFromStore(ctx context.Context, store battleStore, sessionID string) (BattleStateSnapshot, error) {
	session, err := store.GetBattleSession(ctx, sessionID)
	if err != nil {
		return BattleStateSnapshot{}, err
	}
	units, err := store.ListBattleUnits(ctx, sessionID)
	if err != nil {
		return BattleStateSnapshot{}, err
	}
	projectiles, err := store.ListBattleProjectiles(ctx, sessionID)
	if err != nil {
		return BattleStateSnapshot{}, err
	}
	events, err := store.ListBattleEvents(ctx, sessionID, 80)
	if err != nil {
		return BattleStateSnapshot{}, err
	}
	targets, err := store.ListRadarTargets(ctx, sessionID)
	if err != nil {
		return BattleStateSnapshot{}, err
	}
	updatedAt := session.UpdatedAt
	if session.LastScanAt != nil {
		updatedAt = *session.LastScanAt
	}
	if updatedAt.IsZero() {
		updatedAt = session.StartedAt
	}
	return BattleStateSnapshot{
		SessionID:    session.SessionID,
		Status:       session.Status,
		Units:        units,
		Projectiles:  projectiles,
		Events:       events,
		RadarTargets: targets,
		UpdatedAt:    updatedAt,
	}, nil
}

func (s *AppService) ReceiveRadarReport(ctx context.Context, report RadarReportPayload) (BattleStateSnapshot, error) {
	return receiveRadarReportWithDeps(ctx, s.store, s.hub, report)
}

func receiveRadarReportWithDeps(ctx context.Context, store battleStore, broadcaster eventBroadcaster, report RadarReportPayload) (BattleStateSnapshot, error) {
	if err := validateRadarReport(report); err != nil {
		return BattleStateSnapshot{}, err
	}
	scanTime := time.Now()
	if report.ScanTime != nil {
		scanTime = *report.ScanTime
	}
	if _, err := store.GetBattleSession(ctx, report.SessionID); err != nil {
		return BattleStateSnapshot{}, err
	}
	targets := make([]models.RadarTarget, 0, len(report.Targets))
	for _, target := range report.Targets {
		if target.TargetID == "" {
			continue
		}
		targets = append(targets, models.RadarTarget{
			SessionID:  report.SessionID,
			RadarID:    report.RadarID,
			TargetID:   target.TargetID,
			Side:       target.Side,
			Longitude:  target.Longitude,
			Latitude:   target.Latitude,
			Course:     target.Course,
			SpeedKnots: target.SpeedKnots,
			Confidence: target.Confidence,
			Detected:   target.Detected,
			ScanTime:   scanTime,
		})
	}
	if err := store.UpsertRadarTargets(ctx, targets); err != nil {
		return BattleStateSnapshot{}, err
	}
	status := ""
	updatedAt := scanTime
	var projectiles []models.BattleProjectile
	var events []models.BattleEvent
	var insertedEvents []models.BattleEvent
	if report.State != nil {
		if report.State.Status != "" {
			status = report.State.Status
		}
		if report.State.UpdatedAt != nil {
			updatedAt = *report.State.UpdatedAt
		}
		units := battleUnitsFromPayload(report.SessionID, report.State.Units)
		projectiles = battleProjectilesFromPayload(report.SessionID, report.State.Projectiles)
		events = battleEventsFromPayload(report.SessionID, report.State.Events, updatedAt)
		if err := store.UpsertBattleUnits(ctx, units); err != nil {
			return BattleStateSnapshot{}, err
		}
		if err := store.UpsertBattleProjectiles(ctx, projectiles); err != nil {
			return BattleStateSnapshot{}, err
		}
		var err error
		insertedEvents, err = store.CreateBattleEvents(ctx, events)
		if err != nil {
			return BattleStateSnapshot{}, err
		}
	}
	if err := store.UpdateBattleSessionScan(ctx, report.SessionID, status, updatedAt); err != nil {
		return BattleStateSnapshot{}, err
	}
	state, err := getBattleStateFromStore(ctx, store, report.SessionID)
	if err != nil {
		return BattleStateSnapshot{}, err
	}
	if report.State != nil {
		if _, err := saveBattleSnapshotToStore(ctx, store, state, events); err != nil {
			return BattleStateSnapshot{}, err
		}
	}
	broadcastIfPresent(broadcaster, ws.Event{Type: "radar_scan_updated", Data: ginRadarScan(report.SessionID, report.RadarID, scanTime, targets)})
	for _, projectile := range projectiles {
		broadcastIfPresent(broadcaster, ws.Event{Type: "projectile_updated", Data: projectile})
	}
	for _, event := range insertedEvents {
		broadcastIfPresent(broadcaster, ws.Event{Type: "battle_event_created", Data: event})
	}
	if report.State != nil {
		broadcastIfPresent(broadcaster, ws.Event{Type: "battle_state_updated", Data: state})
	}
	return state, nil
}

func (s *AppService) ListBattleTimeline(ctx context.Context, sessionID string) ([]BattleTimelineItem, error) {
	if _, err := s.store.GetBattleSession(ctx, sessionID); err != nil {
		return nil, err
	}
	snapshots, err := s.store.ListBattleSnapshotTimeline(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	items := make([]BattleTimelineItem, 0, len(snapshots))
	for _, snapshot := range snapshots {
		events, err := decodeSnapshotEvents(snapshot.EventsJSON)
		if err != nil {
			return nil, err
		}
		summaries := make([]BattleTimelineEventSummary, 0, minInt(len(events), 4))
		for _, event := range events {
			if len(summaries) >= 4 {
				break
			}
			summaries = append(summaries, BattleTimelineEventSummary{
				Type:         event.Type,
				Severity:     event.Severity,
				Message:      event.Message,
				SourceUnitID: event.SourceUnitID,
				TargetUnitID: event.TargetUnitID,
			})
		}
		items = append(items, BattleTimelineItem{
			Tick:         snapshot.Tick,
			SnapshotTime: snapshot.SnapshotTime,
			EventCount:   len(events),
			Events:       summaries,
		})
	}
	return items, nil
}

func (s *AppService) ListBattleSnapshots(ctx context.Context, sessionID string, fromTick, toTick *int64) ([]BattleSnapshotView, error) {
	if _, err := s.store.GetBattleSession(ctx, sessionID); err != nil {
		return nil, err
	}
	snapshots, err := s.store.ListBattleSnapshots(ctx, sessionID, fromTick, toTick)
	if err != nil {
		return nil, err
	}
	views := make([]BattleSnapshotView, 0, len(snapshots))
	for _, snapshot := range snapshots {
		view, err := decodeBattleSnapshot(snapshot)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *AppService) GetBattleReport(ctx context.Context, sessionID string) (BattleReport, error) {
	session, err := s.store.GetBattleSession(ctx, sessionID)
	if err != nil {
		return BattleReport{}, err
	}
	snapshots, err := s.store.ListBattleSnapshots(ctx, sessionID, nil, nil)
	if err != nil {
		return BattleReport{}, err
	}
	var allEvents []models.BattleEvent
	var latestUnits []models.BattleUnit
	for _, snapshot := range snapshots {
		events, err := decodeSnapshotEvents(snapshot.EventsJSON)
		if err != nil {
			return BattleReport{}, err
		}
		allEvents = append(allEvents, events...)
		units, err := decodeSnapshotUnits(snapshot.UnitsJSON)
		if err != nil {
			return BattleReport{}, err
		}
		if len(units) > 0 {
			latestUnits = units
		}
	}
	if len(allEvents) == 0 {
		allEvents, err = s.store.ListBattleEvents(ctx, sessionID, 200)
		if err != nil {
			return BattleReport{}, err
		}
	}
	if len(latestUnits) == 0 {
		latestUnits, err = s.store.ListBattleUnits(ctx, sessionID)
		if err != nil {
			return BattleReport{}, err
		}
	}
	return buildBattleReport(session, allEvents, latestUnits), nil
}

func buildBattleReport(session models.BattleSession, events []models.BattleEvent, units []models.BattleUnit) BattleReport {
	report := BattleReport{
		Session:       session,
		Winner:        inferBattleWinner(session.Status, events),
		DamageRanking: buildDamageRanking(units),
	}
	seenEvents := make(map[string]struct{})
	for _, event := range events {
		key := event.EventID
		if key == "" {
			key = fmt.Sprintf("%s-%s-%d", event.Type, event.Message, event.OccurredAt.UnixNano())
		}
		if _, ok := seenEvents[key]; ok {
			continue
		}
		seenEvents[key] = struct{}{}
		switch event.Type {
		case "WEAPON_FIRED":
			report.FiredCount++
		case "PROJECTILE_HIT":
			report.HitCount++
		case "UNIT_DESTROYED":
			report.DestroyedCount++
		}
		if isKeyBattleEvent(event) {
			report.KeyEvents = append(report.KeyEvents, event)
		}
	}
	sort.Slice(report.KeyEvents, func(i, j int) bool {
		return report.KeyEvents[i].OccurredAt.After(report.KeyEvents[j].OccurredAt)
	})
	if len(report.KeyEvents) > 20 {
		report.KeyEvents = report.KeyEvents[:20]
	}
	return report
}

func (s *AppService) saveBattleSnapshot(ctx context.Context, state BattleStateSnapshot, tickEvents []models.BattleEvent) (bool, error) {
	return saveBattleSnapshotToStore(ctx, s.store, state, tickEvents)
}

func saveBattleSnapshotToStore(ctx context.Context, store battleStore, state BattleStateSnapshot, tickEvents []models.BattleEvent) (bool, error) {
	unitsJSON, err := marshalSnapshotJSON(state.Units)
	if err != nil {
		return false, err
	}
	projectilesJSON, err := marshalSnapshotJSON(state.Projectiles)
	if err != nil {
		return false, err
	}
	targetsJSON, err := marshalSnapshotJSON(state.RadarTargets)
	if err != nil {
		return false, err
	}
	eventsJSON, err := marshalSnapshotJSON(tickEvents)
	if err != nil {
		return false, err
	}
	snapshotTime := state.UpdatedAt
	if snapshotTime.IsZero() {
		snapshotTime = time.Now()
	}
	return store.AppendBattleSnapshot(ctx, &models.BattleSnapshot{
		SessionID:        state.SessionID,
		SnapshotTime:     snapshotTime,
		UnitsJSON:        unitsJSON,
		ProjectilesJSON:  projectilesJSON,
		RadarTargetsJSON: targetsJSON,
		EventsJSON:       eventsJSON,
	})
}

func broadcastIfPresent(broadcaster eventBroadcaster, event ws.Event) {
	if broadcaster == nil {
		return
	}
	broadcaster.Broadcast(event)
}

func marshalSnapshotJSON(value interface{}) (models.JSONB, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return models.JSONB(data), nil
}

func decodeBattleSnapshot(snapshot models.BattleSnapshot) (BattleSnapshotView, error) {
	units, err := decodeSnapshotUnits(snapshot.UnitsJSON)
	if err != nil {
		return BattleSnapshotView{}, err
	}
	projectiles, err := decodeSnapshotProjectiles(snapshot.ProjectilesJSON)
	if err != nil {
		return BattleSnapshotView{}, err
	}
	targets, err := decodeSnapshotRadarTargets(snapshot.RadarTargetsJSON)
	if err != nil {
		return BattleSnapshotView{}, err
	}
	events, err := decodeSnapshotEvents(snapshot.EventsJSON)
	if err != nil {
		return BattleSnapshotView{}, err
	}
	return BattleSnapshotView{
		ID:           snapshot.ID,
		SessionID:    snapshot.SessionID,
		Tick:         snapshot.Tick,
		SnapshotTime: snapshot.SnapshotTime,
		Units:        units,
		Projectiles:  projectiles,
		RadarTargets: targets,
		Events:       events,
	}, nil
}

func decodeSnapshotUnits(data models.JSONB) ([]models.BattleUnit, error) {
	var items []models.BattleUnit
	err := json.Unmarshal(data, &items)
	return items, err
}

func decodeSnapshotProjectiles(data models.JSONB) ([]models.BattleProjectile, error) {
	var items []models.BattleProjectile
	err := json.Unmarshal(data, &items)
	return items, err
}

func decodeSnapshotRadarTargets(data models.JSONB) ([]models.RadarTarget, error) {
	var items []models.RadarTarget
	err := json.Unmarshal(data, &items)
	return items, err
}

func decodeSnapshotEvents(data models.JSONB) ([]models.BattleEvent, error) {
	var items []models.BattleEvent
	err := json.Unmarshal(data, &items)
	return items, err
}

func buildDamageRanking(units []models.BattleUnit) []BattleDamageStat {
	ranking := make([]BattleDamageStat, 0, len(units))
	for _, unit := range units {
		damage := unit.MaxHP - unit.HP
		if damage < 0 {
			damage = 0
		}
		ranking = append(ranking, BattleDamageStat{
			UnitID:      unit.UnitID,
			Name:        unit.Name,
			Side:        unit.Side,
			MaxHP:       unit.MaxHP,
			HP:          unit.HP,
			DamageTaken: damage,
			Status:      unit.Status,
		})
	}
	sort.Slice(ranking, func(i, j int) bool {
		if ranking[i].DamageTaken == ranking[j].DamageTaken {
			return ranking[i].UnitID < ranking[j].UnitID
		}
		return ranking[i].DamageTaken > ranking[j].DamageTaken
	})
	return ranking
}

func inferBattleWinner(status string, events []models.BattleEvent) string {
	switch status {
	case "blue_victory":
		return "blue"
	case "red_victory":
		return "red"
	case "running":
		return "pending"
	}
	for _, event := range events {
		if event.Type != "BATTLE_ENDED" {
			continue
		}
		if containsFold(event.Message, "Blue") {
			return "blue"
		}
		if containsFold(event.Message, "Red") {
			return "red"
		}
	}
	return "undecided"
}

func containsFold(value, needle string) bool {
	value = strings.ToLower(value)
	needle = strings.ToLower(needle)
	return strings.Contains(value, needle)
}

func isKeyBattleEvent(event models.BattleEvent) bool {
	switch event.Type {
	case "PROJECTILE_HIT", "UNIT_DESTROYED", "BATTLE_ENDED":
		return true
	}
	return event.Severity == "CRITICAL"
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *AppService) findScenario(code string) (BattleScenario, error) {
	code = strings.TrimSpace(code)
	scenarios := s.ListBattleScenarios(context.Background())
	if code == "" && len(scenarios) > 0 {
		return scenarios[0], nil
	}
	for _, scenario := range scenarios {
		if scenario.Code == code {
			return scenario, nil
		}
	}
	return BattleScenario{}, &ValidationError{message: "battle scenario does not exist"}
}

func battleUnitsFromPayload(sessionID string, payload []BattleUnitPayload) []models.BattleUnit {
	units := make([]models.BattleUnit, 0, len(payload))
	for _, unit := range payload {
		if unit.UnitID == "" {
			continue
		}
		status := unit.Status
		if status == "" {
			status = "active"
		}
		units = append(units, models.BattleUnit{
			SessionID:       sessionID,
			UnitID:          unit.UnitID,
			ShipID:          unit.ShipID,
			Name:            unit.Name,
			Side:            unit.Side,
			HP:              unit.HP,
			MaxHP:           unit.MaxHP,
			RadarRangeKm:    unit.RadarRangeKm,
			WeaponRangeKm:   unit.WeaponRangeKm,
			CooldownSeconds: unit.CooldownSeconds,
			Longitude:       unit.Longitude,
			Latitude:        unit.Latitude,
			Course:          unit.Course,
			SpeedKnots:      unit.SpeedKnots,
			Status:          status,
		})
	}
	return units
}

func battleProjectilesFromPayload(sessionID string, payload []BattleProjectilePayload) []models.BattleProjectile {
	projectiles := make([]models.BattleProjectile, 0, len(payload))
	for _, projectile := range payload {
		if projectile.ProjectileID == "" {
			continue
		}
		projectiles = append(projectiles, models.BattleProjectile{
			SessionID:    sessionID,
			ProjectileID: projectile.ProjectileID,
			SourceUnitID: projectile.SourceUnitID,
			TargetUnitID: projectile.TargetUnitID,
			Side:         projectile.Side,
			Longitude:    projectile.Longitude,
			Latitude:     projectile.Latitude,
			SpeedKmH:     projectile.SpeedKmH,
			Status:       projectile.Status,
		})
	}
	return projectiles
}

func battleEventsFromPayload(sessionID string, payload []BattleEventPayload, fallbackTime time.Time) []models.BattleEvent {
	events := make([]models.BattleEvent, 0, len(payload))
	for index, event := range payload {
		if event.Type == "" || event.Message == "" {
			continue
		}
		occurredAt := fallbackTime
		if event.OccurredAt != nil {
			occurredAt = *event.OccurredAt
		}
		eventID := event.EventID
		if eventID == "" {
			eventID = fmt.Sprintf("%s-%s-%d-%d", sessionID, event.Type, occurredAt.UnixNano(), index)
		}
		severity := event.Severity
		if severity == "" {
			severity = "INFO"
		}
		events = append(events, models.BattleEvent{
			SessionID:    sessionID,
			EventID:      eventID,
			Type:         event.Type,
			Severity:     severity,
			Message:      event.Message,
			SourceUnitID: event.SourceUnitID,
			TargetUnitID: event.TargetUnitID,
			Longitude:    event.Longitude,
			Latitude:     event.Latitude,
			OccurredAt:   occurredAt,
		})
	}
	return events
}

func ginRadarScan(sessionID, radarID string, scanTime time.Time, targets []models.RadarTarget) map[string]interface{} {
	return map[string]interface{}{
		"sessionId": sessionID,
		"radarId":   radarID,
		"scanTime":  scanTime,
		"targets":   targets,
	}
}

func normalizePage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

func normalizeSize(size int) int {
	if size < 1 {
		return 20
	}
	if size > 100 {
		return 100
	}
	return size
}

func validateShip(ship *models.Ship) error {
	ship.Name = strings.TrimSpace(ship.Name)
	ship.MMSI = strings.TrimSpace(ship.MMSI)
	ship.ShipType = strings.TrimSpace(ship.ShipType)
	ship.Flag = strings.TrimSpace(ship.Flag)
	ship.Status = strings.TrimSpace(ship.Status)
	if ship.Name == "" || ship.MMSI == "" {
		return &ValidationError{message: "船名和 MMSI 必填"}
	}
	if !validMMSI(ship.MMSI) {
		return &ValidationError{message: "MMSI 必须为 9 位数字"}
	}
	if ship.LengthM < 0 || ship.LengthM > 500 || ship.WidthM < 0 || ship.WidthM > 100 {
		return &ValidationError{message: "船舶尺度超出合法范围"}
	}
	if ship.Status == "" {
		ship.Status = "active"
	}
	if !validShipStatus(ship.Status) {
		return &ValidationError{message: "船舶状态不合法"}
	}
	return nil
}

func validMMSI(value string) bool {
	if len(value) != 9 {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func validShipStatus(value string) bool {
	switch value {
	case "active", "inactive", "maintenance":
		return true
	default:
		return false
	}
}

func validateRadarReport(report RadarReportPayload) error {
	if strings.TrimSpace(report.SessionID) == "" || strings.TrimSpace(report.RadarID) == "" {
		return &ValidationError{message: "sessionId and radarId are required"}
	}
	if len(report.Targets) > maxRadarTargetsPerReport {
		return &ValidationError{message: fmt.Sprintf("radar report targets cannot exceed %d", maxRadarTargetsPerReport)}
	}
	for _, target := range report.Targets {
		if strings.TrimSpace(target.TargetID) == "" {
			return &ValidationError{message: "radar targetId is required"}
		}
		if !validSide(target.Side) || !validLongitude(target.Longitude) || !validLatitude(target.Latitude) ||
			!validCourse(target.Course) || target.SpeedKnots < 0 || !validConfidence(target.Confidence) {
			return &ValidationError{message: "radar target contains invalid navigation values"}
		}
	}
	if report.State == nil {
		return nil
	}
	if report.State.SessionID != "" && report.State.SessionID != report.SessionID {
		return &ValidationError{message: "battle state sessionId must match radar report sessionId"}
	}
	if report.State.Status != "" && !validBattleSessionStatus(report.State.Status) {
		return &ValidationError{message: "battle state status is invalid"}
	}
	if len(report.State.Units) > maxBattleUnitsPerReport {
		return &ValidationError{message: fmt.Sprintf("battle units cannot exceed %d", maxBattleUnitsPerReport)}
	}
	if len(report.State.Projectiles) > maxBattleProjectilesPerReport {
		return &ValidationError{message: fmt.Sprintf("battle projectiles cannot exceed %d", maxBattleProjectilesPerReport)}
	}
	if len(report.State.Events) > maxBattleEventsPerReport {
		return &ValidationError{message: fmt.Sprintf("battle events cannot exceed %d", maxBattleEventsPerReport)}
	}
	for _, unit := range report.State.Units {
		if strings.TrimSpace(unit.UnitID) == "" || strings.TrimSpace(unit.Name) == "" {
			return &ValidationError{message: "battle unitId and name are required"}
		}
		if !validSide(unit.Side) || !validLongitude(unit.Longitude) || !validLatitude(unit.Latitude) ||
			!validCourse(unit.Course) || unit.SpeedKnots < 0 || unit.HP < 0 || unit.MaxHP < 0 ||
			unit.RadarRangeKm < 0 || unit.WeaponRangeKm < 0 || unit.CooldownSeconds < 0 ||
			(unit.Status != "" && !validBattleUnitStatus(unit.Status)) {
			return &ValidationError{message: "battle unit contains invalid values"}
		}
	}
	for _, projectile := range report.State.Projectiles {
		if strings.TrimSpace(projectile.ProjectileID) == "" || strings.TrimSpace(projectile.SourceUnitID) == "" ||
			strings.TrimSpace(projectile.TargetUnitID) == "" {
			return &ValidationError{message: "battle projectile id, sourceUnitId and targetUnitId are required"}
		}
		if !validSide(projectile.Side) || !validLongitude(projectile.Longitude) || !validLatitude(projectile.Latitude) ||
			projectile.SpeedKmH < 0 || !validProjectileStatus(projectile.Status) {
			return &ValidationError{message: "battle projectile contains invalid values"}
		}
	}
	for _, event := range report.State.Events {
		if strings.TrimSpace(event.Type) == "" || strings.TrimSpace(event.Message) == "" {
			return &ValidationError{message: "battle event type and message are required"}
		}
		if event.Severity != "" && !validEventSeverity(event.Severity) {
			return &ValidationError{message: "battle event severity is invalid"}
		}
		if (event.Longitude == nil) != (event.Latitude == nil) {
			return &ValidationError{message: "battle event longitude and latitude must be provided together"}
		}
		if event.Longitude != nil && (!validLongitude(*event.Longitude) || !validLatitude(*event.Latitude)) {
			return &ValidationError{message: "battle event coordinates are invalid"}
		}
	}
	return nil
}

func validLongitude(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= -180 && value <= 180
}

func validLatitude(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= -90 && value <= 90
}

func validCourse(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value < 360
}

func validConfidence(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1
}

func validSide(value string) bool {
	switch value {
	case "blue", "red", "neutral", "unknown":
		return true
	default:
		return false
	}
}

func validBattleSessionStatus(value string) bool {
	switch value {
	case "running", "blue_victory", "red_victory", "stopped":
		return true
	default:
		return false
	}
}

func validBattleUnitStatus(value string) bool {
	switch value {
	case "active", "destroyed", "disabled":
		return true
	default:
		return false
	}
}

func validProjectileStatus(value string) bool {
	switch value {
	case "flying", "hit", "expired", "missed":
		return true
	default:
		return false
	}
}

func validEventSeverity(value string) bool {
	switch value {
	case "INFO", "WARN", "CRITICAL":
		return true
	default:
		return false
	}
}

func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func IsValidationError(err error) bool {
	var pointerValidationErr *ValidationError
	if errors.As(err, &pointerValidationErr) {
		return true
	}
	var valueValidationErr ValidationError
	return errors.As(err, &valueValidationErr)
}

func IsAuthenticationError(err error) bool {
	var pointerAuthenticationErr *AuthenticationError
	if errors.As(err, &pointerAuthenticationErr) {
		return true
	}
	var valueAuthenticationErr AuthenticationError
	return errors.As(err, &valueAuthenticationErr)
}

func isAlarmStatus(status string) bool {
	switch status {
	case "OPEN", "ACKED":
		return true
	default:
		return false
	}
}

func isDispatchStatus(status string) bool {
	switch status {
	case "NEW", "DISPATCHED", "PROCESSING", "COMPLETED", "CANCELLED":
		return true
	default:
		return false
	}
}

func isDispatchPriority(priority string) bool {
	switch priority {
	case "low", "normal", "high":
		return true
	default:
		return false
	}
}

func canTransitionDispatch(from, to string) bool {
	if from == to {
		return true
	}
	switch from {
	case "NEW":
		return to == "DISPATCHED" || to == "PROCESSING" || to == "CANCELLED"
	case "DISPATCHED":
		return to == "PROCESSING" || to == "COMPLETED" || to == "CANCELLED"
	case "PROCESSING":
		return to == "COMPLETED" || to == "CANCELLED"
	case "COMPLETED", "CANCELLED":
		return false
	default:
		return false
	}
}

func normalizeDispatchStatusUpdate(fromStatus, toStatus, remark string) (string, string, bool) {
	toStatus = strings.TrimSpace(toStatus)
	remark = strings.TrimSpace(remark)
	return toStatus, remark, fromStatus != toStatus
}
