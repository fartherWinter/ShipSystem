package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

type JSONB json.RawMessage

func (j JSONB) Value() (driver.Value, error) {
	if len(j) == 0 {
		return "[]", nil
	}
	if !json.Valid(j) {
		return nil, errors.New("invalid jsonb value")
	}
	return string(j), nil
}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = JSONB("[]")
		return nil
	}
	switch data := value.(type) {
	case []byte:
		*j = append((*j)[0:0], data...)
	case string:
		*j = append((*j)[0:0], data...)
	default:
		return errors.New("unsupported jsonb scan value")
	}
	return nil
}

func (j JSONB) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("[]"), nil
	}
	return json.RawMessage(j).MarshalJSON()
}

func (j *JSONB) UnmarshalJSON(data []byte) error {
	if !json.Valid(data) {
		return errors.New("invalid jsonb value")
	}
	*j = append((*j)[0:0], data...)
	return nil
}

type Role struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"size:64;not null"`
	Code        string    `json:"code" gorm:"size:64;not null;uniqueIndex"`
	Description string    `json:"description" gorm:"size:255"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Menu struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"size:64;not null"`
	Path      string    `json:"path" gorm:"size:128;not null;uniqueIndex"`
	Icon      string    `json:"icon" gorm:"size:64"`
	ParentID  *uint     `json:"parentId"`
	Sort      int       `json:"sort"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type User struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	Username     string    `json:"username" gorm:"size:64;not null;uniqueIndex"`
	PasswordHash string    `json:"-" gorm:"size:255;not null"`
	DisplayName  string    `json:"displayName" gorm:"size:64;not null"`
	RoleID       uint      `json:"roleId"`
	Role         Role      `json:"role"`
	Status       string    `json:"status" gorm:"size:32;default:enabled"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type Ship struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Name      string         `json:"name" gorm:"size:128;not null"`
	MMSI      string         `json:"mmsi" gorm:"size:32;not null;uniqueIndex"`
	ShipType  string         `json:"shipType" gorm:"size:64"`
	Flag      string         `json:"flag" gorm:"size:32"`
	LengthM   float64        `json:"lengthM"`
	WidthM    float64        `json:"widthM"`
	Status    string         `json:"status" gorm:"size:32;default:active"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

type ShipLocation struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	ShipID     uint      `json:"shipId" gorm:"index;not null"`
	Ship       Ship      `json:"ship"`
	Longitude  float64   `json:"longitude" gorm:"not null"`
	Latitude   float64   `json:"latitude" gorm:"not null"`
	SpeedKnots float64   `json:"speedKnots"`
	Course     float64   `json:"course"`
	ReportedAt time.Time `json:"reportedAt" gorm:"index;not null"`
	CreatedAt  time.Time `json:"createdAt"`
}

type Alarm struct {
	ID        uint       `json:"id" gorm:"primaryKey"`
	ShipID    uint       `json:"shipId" gorm:"index;not null"`
	Ship      Ship       `json:"ship"`
	Type      string     `json:"type" gorm:"size:64;not null"`
	Level     string     `json:"level" gorm:"size:32;not null"`
	Title     string     `json:"title" gorm:"size:128;not null"`
	Message   string     `json:"message" gorm:"size:512"`
	Longitude *float64   `json:"longitude"`
	Latitude  *float64   `json:"latitude"`
	Status    string     `json:"status" gorm:"size:32;default:OPEN;index"`
	AckBy     *uint      `json:"ackBy"`
	AckAt     *time.Time `json:"ackAt"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

type DispatchEvent struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	ShipID        *uint     `json:"shipId"`
	Ship          *Ship     `json:"ship"`
	Title         string    `json:"title" gorm:"size:128;not null"`
	Description   string    `json:"description" gorm:"size:1024"`
	Status        string    `json:"status" gorm:"size:32;default:NEW;index"`
	Priority      string    `json:"priority" gorm:"size:32;default:normal"`
	HandlerUserID *uint     `json:"handlerUserId"`
	CreatedByID   *uint     `json:"createdById"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type DispatchEventLog struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	EventID    uint      `json:"eventId" gorm:"index;not null"`
	FromStatus string    `json:"fromStatus" gorm:"size:32"`
	ToStatus   string    `json:"toStatus" gorm:"size:32;not null"`
	Remark     string    `json:"remark" gorm:"size:512"`
	OperatorID *uint     `json:"operatorId"`
	CreatedAt  time.Time `json:"createdAt"`
}

type BattleSession struct {
	ID           uint       `json:"id" gorm:"primaryKey"`
	SessionID    string     `json:"sessionId" gorm:"size:64;not null;uniqueIndex"`
	Name         string     `json:"name" gorm:"size:128;not null"`
	ScenarioCode string     `json:"scenarioCode" gorm:"size:64;not null;index"`
	Status       string     `json:"status" gorm:"size:32;not null;default:running;index"`
	StartedAt    time.Time  `json:"startedAt" gorm:"index;not null"`
	StoppedAt    *time.Time `json:"stoppedAt"`
	LastScanAt   *time.Time `json:"lastScanAt"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type RadarTarget struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	SessionID  string    `json:"sessionId" gorm:"size:64;not null;index;uniqueIndex:idx_radar_target_current,priority:1"`
	RadarID    string    `json:"radarId" gorm:"size:64;not null;uniqueIndex:idx_radar_target_current,priority:2"`
	TargetID   string    `json:"targetId" gorm:"size:64;not null;uniqueIndex:idx_radar_target_current,priority:3"`
	Side       string    `json:"side" gorm:"size:16;not null;index"`
	Longitude  float64   `json:"longitude" gorm:"not null"`
	Latitude   float64   `json:"latitude" gorm:"not null"`
	Course     float64   `json:"course"`
	SpeedKnots float64   `json:"speedKnots"`
	Confidence float64   `json:"confidence"`
	Detected   bool      `json:"detected" gorm:"index"`
	ScanTime   time.Time `json:"scanTime" gorm:"index;not null"`
	CreatedAt  time.Time `json:"createdAt"`
}

type BattleUnit struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	SessionID       string    `json:"sessionId" gorm:"size:64;not null;index;uniqueIndex:idx_battle_unit_current,priority:1"`
	UnitID          string    `json:"unitId" gorm:"size:64;not null;uniqueIndex:idx_battle_unit_current,priority:2"`
	ShipID          uint      `json:"shipId"`
	Name            string    `json:"name" gorm:"size:128;not null"`
	Side            string    `json:"side" gorm:"size:16;not null;index"`
	HP              float64   `json:"hp"`
	MaxHP           float64   `json:"maxHp"`
	RadarRangeKm    float64   `json:"radarRangeKm"`
	WeaponRangeKm   float64   `json:"weaponRangeKm"`
	CooldownSeconds float64   `json:"cooldownSeconds"`
	Longitude       float64   `json:"longitude" gorm:"not null"`
	Latitude        float64   `json:"latitude" gorm:"not null"`
	Course          float64   `json:"course"`
	SpeedKnots      float64   `json:"speedKnots"`
	Status          string    `json:"status" gorm:"size:32;not null;default:active;index"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type BattleProjectile struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	SessionID    string    `json:"sessionId" gorm:"size:64;not null;index;uniqueIndex:idx_battle_projectile_current,priority:1"`
	ProjectileID string    `json:"projectileId" gorm:"size:64;not null;uniqueIndex:idx_battle_projectile_current,priority:2"`
	SourceUnitID string    `json:"sourceUnitId" gorm:"size:64;not null;index"`
	TargetUnitID string    `json:"targetUnitId" gorm:"size:64;not null;index"`
	Side         string    `json:"side" gorm:"size:16;not null;index"`
	Longitude    float64   `json:"longitude" gorm:"not null"`
	Latitude     float64   `json:"latitude" gorm:"not null"`
	SpeedKmH     float64   `json:"speedKmH"`
	Status       string    `json:"status" gorm:"size:32;not null;index"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type BattleEvent struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	SessionID    string    `json:"sessionId" gorm:"size:64;not null;index;uniqueIndex:idx_battle_event_once,priority:1"`
	EventID      string    `json:"eventId" gorm:"size:96;not null;uniqueIndex:idx_battle_event_once,priority:2"`
	Type         string    `json:"type" gorm:"size:64;not null;index"`
	Severity     string    `json:"severity" gorm:"size:32;not null;index"`
	Message      string    `json:"message" gorm:"size:512;not null"`
	SourceUnitID string    `json:"sourceUnitId" gorm:"size:64"`
	TargetUnitID string    `json:"targetUnitId" gorm:"size:64"`
	Longitude    *float64  `json:"longitude"`
	Latitude     *float64  `json:"latitude"`
	OccurredAt   time.Time `json:"occurredAt" gorm:"index;not null"`
	CreatedAt    time.Time `json:"createdAt"`
}

type BattleSnapshot struct {
	ID               uint      `json:"id" gorm:"primaryKey"`
	SessionID        string    `json:"sessionId" gorm:"size:64;not null;index;uniqueIndex:idx_battle_snapshot_tick,priority:1;index:idx_battle_snapshots_session_time,priority:1"`
	Tick             int64     `json:"tick" gorm:"not null;uniqueIndex:idx_battle_snapshot_tick,priority:2"`
	SnapshotTime     time.Time `json:"snapshotTime" gorm:"not null;index:idx_battle_snapshots_session_time,priority:2"`
	UnitsJSON        JSONB     `json:"units" gorm:"type:jsonb;not null"`
	ProjectilesJSON  JSONB     `json:"projectiles" gorm:"type:jsonb;not null"`
	RadarTargetsJSON JSONB     `json:"radarTargets" gorm:"type:jsonb;not null"`
	EventsJSON       JSONB     `json:"events" gorm:"type:jsonb;not null"`
	CreatedAt        time.Time `json:"createdAt"`
}
