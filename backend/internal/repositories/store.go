package repositories

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"shipsystem/backend/internal/models"
)

type Store struct {
	DB *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{DB: db}
}

func (s *Store) FindUserByUsername(ctx context.Context, username string) (models.User, error) {
	var user models.User
	err := s.DB.WithContext(ctx).Preload("Role").Where("username = ?", username).First(&user).Error
	return user, err
}

func (s *Store) ListMenus(ctx context.Context) ([]models.Menu, error) {
	var menus []models.Menu
	err := s.DB.WithContext(ctx).Order("sort asc").Find(&menus).Error
	return menus, err
}

func (s *Store) ListUsers(ctx context.Context) ([]models.User, error) {
	var users []models.User
	err := s.DB.WithContext(ctx).Preload("Role").Order("id asc").Find(&users).Error
	return users, err
}

func (s *Store) ListRoles(ctx context.Context) ([]models.Role, error) {
	var roles []models.Role
	err := s.DB.WithContext(ctx).Order("id asc").Find(&roles).Error
	return roles, err
}

func (s *Store) ListShips(ctx context.Context, keyword string, page, size int) ([]models.Ship, int64, error) {
	var ships []models.Ship
	var total int64
	query := s.DB.WithContext(ctx).Model(&models.Ship{})
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name ILIKE ? OR mmsi ILIKE ?", like, like)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&ships).Error
	return ships, total, err
}

func (s *Store) GetShip(ctx context.Context, id uint) (models.Ship, error) {
	var ship models.Ship
	err := s.DB.WithContext(ctx).First(&ship, id).Error
	return ship, err
}

func (s *Store) CreateShip(ctx context.Context, ship *models.Ship) error {
	return s.DB.WithContext(ctx).Create(ship).Error
}

func (s *Store) UpdateShip(ctx context.Context, ship *models.Ship) error {
	return s.DB.WithContext(ctx).Save(ship).Error
}

func (s *Store) DeleteShip(ctx context.Context, id uint) error {
	return s.DB.WithContext(ctx).Delete(&models.Ship{}, id).Error
}

func (s *Store) CreateLocation(ctx context.Context, loc *models.ShipLocation) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ship models.Ship
		if err := tx.First(&ship, loc.ShipID).Error; err != nil {
			return err
		}
		if err := tx.Create(loc).Error; err != nil {
			return err
		}
		return tx.Exec(
			`UPDATE ship_locations SET geom = ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography WHERE id = ?`,
			loc.Longitude,
			loc.Latitude,
			loc.ID,
		).Error
	})
}

func (s *Store) LatestLocation(ctx context.Context, shipID uint) (models.ShipLocation, error) {
	var loc models.ShipLocation
	err := s.DB.WithContext(ctx).Where("ship_id = ?", shipID).Order("reported_at desc").First(&loc).Error
	return loc, err
}

func (s *Store) ListTracks(ctx context.Context, shipID uint, start, end *time.Time) ([]models.ShipLocation, error) {
	var locations []models.ShipLocation
	query := s.DB.WithContext(ctx).Where("ship_id = ?", shipID)
	if start != nil {
		query = query.Where("reported_at >= ?", *start)
	}
	if end != nil {
		query = query.Where("reported_at <= ?", *end)
	}
	err := query.Order("reported_at asc").Find(&locations).Error
	return locations, err
}

func (s *Store) CreateAlarm(ctx context.Context, alarm *models.Alarm) error {
	return s.DB.WithContext(ctx).Create(alarm).Error
}

func (s *Store) ListAlarms(ctx context.Context, status string, page, size int) ([]models.Alarm, int64, error) {
	var alarms []models.Alarm
	var total int64
	query := s.DB.WithContext(ctx).Model(&models.Alarm{}).Preload("Ship")
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&alarms).Error
	return alarms, total, err
}

func (s *Store) AckAlarm(ctx context.Context, id, userID uint) (models.Alarm, bool, error) {
	var alarm models.Alarm
	changed := false
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&alarm, id).Error; err != nil {
			return err
		}
		if alarm.Status == "ACKED" {
			return nil
		}
		now := time.Now()
		alarm.Status = "ACKED"
		alarm.AckBy = &userID
		alarm.AckAt = &now
		if err := tx.Save(&alarm).Error; err != nil {
			return err
		}
		changed = true
		return nil
	})
	return alarm, changed, err
}

func (s *Store) ListDispatchEvents(ctx context.Context, status string, page, size int) ([]models.DispatchEvent, int64, error) {
	var events []models.DispatchEvent
	var total int64
	query := s.DB.WithContext(ctx).Model(&models.DispatchEvent{}).Preload("Ship")
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&events).Error
	return events, total, err
}

func (s *Store) CreateDispatchEvent(ctx context.Context, event *models.DispatchEvent) error {
	return s.DB.WithContext(ctx).Create(event).Error
}

func (s *Store) GetDispatchEvent(ctx context.Context, id uint) (models.DispatchEvent, error) {
	var event models.DispatchEvent
	err := s.DB.WithContext(ctx).First(&event, id).Error
	return event, err
}

func (s *Store) UpdateDispatchStatus(ctx context.Context, id uint, toStatus, remark string, operatorID *uint) (models.DispatchEvent, bool, error) {
	var event models.DispatchEvent
	changed := false
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&event, id).Error; err != nil {
			return err
		}
		fromStatus := event.Status
		if fromStatus == toStatus {
			return nil
		}
		event.Status = toStatus
		if err := tx.Save(&event).Error; err != nil {
			return err
		}
		if err := tx.Create(&models.DispatchEventLog{
			EventID:    event.ID,
			FromStatus: fromStatus,
			ToStatus:   toStatus,
			Remark:     remark,
			OperatorID: operatorID,
		}).Error; err != nil {
			return err
		}
		changed = true
		return nil
	})
	return event, changed, err
}

func (s *Store) CreateBattleSession(ctx context.Context, session *models.BattleSession) error {
	return s.DB.WithContext(ctx).Create(session).Error
}

func (s *Store) ListBattleSessions(ctx context.Context, page, size int) ([]models.BattleSession, int64, error) {
	var sessions []models.BattleSession
	var total int64
	query := s.DB.WithContext(ctx).Model(&models.BattleSession{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("started_at desc").Offset((page - 1) * size).Limit(size).Find(&sessions).Error
	return sessions, total, err
}

func (s *Store) GetBattleSession(ctx context.Context, sessionID string) (models.BattleSession, error) {
	var session models.BattleSession
	err := s.DB.WithContext(ctx).Where("session_id = ?", sessionID).First(&session).Error
	return session, err
}

func (s *Store) StopBattleSession(ctx context.Context, sessionID string) (models.BattleSession, bool, error) {
	var session models.BattleSession
	changed := false
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("session_id = ?", sessionID).First(&session).Error; err != nil {
			return err
		}
		if session.Status == "stopped" {
			return nil
		}
		now := time.Now()
		session.Status = "stopped"
		session.StoppedAt = &now
		if err := tx.Save(&session).Error; err != nil {
			return err
		}
		changed = true
		return nil
	})
	return session, changed, err
}

func (s *Store) UpdateBattleSessionScan(ctx context.Context, sessionID, status string, scanTime time.Time) error {
	updates := map[string]interface{}{"last_scan_at": scanTime}
	if status != "" {
		updates["status"] = status
	}
	return s.DB.WithContext(ctx).Model(&models.BattleSession{}).Where("session_id = ?", sessionID).Updates(updates).Error
}

func (s *Store) UpsertRadarTargets(ctx context.Context, targets []models.RadarTarget) error {
	if len(targets) == 0 {
		return nil
	}
	return s.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "session_id"}, {Name: "radar_id"}, {Name: "target_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"side",
			"longitude",
			"latitude",
			"course",
			"speed_knots",
			"confidence",
			"detected",
			"scan_time",
		}),
	}).Create(&targets).Error
}

func (s *Store) UpsertBattleUnits(ctx context.Context, units []models.BattleUnit) error {
	if len(units) == 0 {
		return nil
	}
	return s.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "session_id"}, {Name: "unit_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"ship_id",
			"name",
			"side",
			"hp",
			"max_hp",
			"radar_range_km",
			"weapon_range_km",
			"cooldown_seconds",
			"longitude",
			"latitude",
			"course",
			"speed_knots",
			"status",
			"updated_at",
		}),
	}).Create(&units).Error
}

func (s *Store) UpsertBattleProjectiles(ctx context.Context, projectiles []models.BattleProjectile) error {
	if len(projectiles) == 0 {
		return nil
	}
	return s.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "session_id"}, {Name: "projectile_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"source_unit_id",
			"target_unit_id",
			"side",
			"longitude",
			"latitude",
			"speed_km_h",
			"status",
			"updated_at",
		}),
	}).Create(&projectiles).Error
}

func (s *Store) CreateBattleEvents(ctx context.Context, events []models.BattleEvent) ([]models.BattleEvent, error) {
	if len(events) == 0 {
		return nil, nil
	}
	if err := s.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&events).Error; err != nil {
		return nil, err
	}
	inserted := make([]models.BattleEvent, 0, len(events))
	for _, event := range events {
		if event.ID != 0 {
			inserted = append(inserted, event)
		}
	}
	return inserted, nil
}

func (s *Store) AppendBattleSnapshot(ctx context.Context, snapshot *models.BattleSnapshot) (bool, error) {
	inserted := false
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var session models.BattleSession
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("session_id = ?", snapshot.SessionID).First(&session).Error; err != nil {
			return err
		}
		var existing int64
		if err := tx.Model(&models.BattleSnapshot{}).
			Where("session_id = ? AND snapshot_time = ?", snapshot.SessionID, snapshot.SnapshotTime).
			Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return nil
		}
		var tick int64
		if err := tx.Model(&models.BattleSnapshot{}).
			Select("COALESCE(MAX(tick), 0) + 1").
			Where("session_id = ?", snapshot.SessionID).
			Scan(&tick).Error; err != nil {
			return err
		}
		snapshot.Tick = tick
		if err := tx.Create(snapshot).Error; err != nil {
			return err
		}
		inserted = true
		return nil
	})
	return inserted, err
}

func (s *Store) ListBattleSnapshots(ctx context.Context, sessionID string, fromTick, toTick *int64) ([]models.BattleSnapshot, error) {
	var snapshots []models.BattleSnapshot
	query := s.DB.WithContext(ctx).Where("session_id = ?", sessionID)
	if fromTick != nil {
		query = query.Where("tick >= ?", *fromTick)
	}
	if toTick != nil {
		query = query.Where("tick <= ?", *toTick)
	}
	err := query.Order("tick asc").Find(&snapshots).Error
	return snapshots, err
}

func (s *Store) ListBattleSnapshotTimeline(ctx context.Context, sessionID string) ([]models.BattleSnapshot, error) {
	var snapshots []models.BattleSnapshot
	err := s.DB.WithContext(ctx).
		Select([]string{"id", "session_id", "tick", "snapshot_time", "events_json", "created_at"}).
		Where("session_id = ?", sessionID).
		Order("tick asc").
		Find(&snapshots).Error
	return snapshots, err
}

func (s *Store) ListBattleUnits(ctx context.Context, sessionID string) ([]models.BattleUnit, error) {
	var units []models.BattleUnit
	err := s.DB.WithContext(ctx).Where("session_id = ?", sessionID).Order("side asc, unit_id asc").Find(&units).Error
	return units, err
}

func (s *Store) ListBattleProjectiles(ctx context.Context, sessionID string) ([]models.BattleProjectile, error) {
	var projectiles []models.BattleProjectile
	err := s.DB.WithContext(ctx).Where("session_id = ?", sessionID).Order("updated_at desc").Find(&projectiles).Error
	return projectiles, err
}

func (s *Store) ListBattleEvents(ctx context.Context, sessionID string, limit int) ([]models.BattleEvent, error) {
	var events []models.BattleEvent
	if limit <= 0 || limit > 200 {
		limit = 80
	}
	err := s.DB.WithContext(ctx).Where("session_id = ?", sessionID).Order("occurred_at desc").Limit(limit).Find(&events).Error
	return events, err
}

func (s *Store) ListRadarTargets(ctx context.Context, sessionID string) ([]models.RadarTarget, error) {
	var targets []models.RadarTarget
	err := s.DB.WithContext(ctx).Where("session_id = ?", sessionID).Order("scan_time desc, target_id asc").Find(&targets).Error
	return targets, err
}
