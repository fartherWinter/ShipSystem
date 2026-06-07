package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"shipsim/internal/model"
)

type Postgres struct {
	pool *pgxpool.Pool
}

const CurrentMigrationVersion = 2

type MigrationStatus struct {
	Current  int
	Required int
}

func (m MigrationStatus) Ready() bool {
	return m.Current >= m.Required
}

func (m MigrationStatus) Error() error {
	if m.Ready() {
		return nil
	}
	return fmt.Errorf("database migrations are not current: current version %d, required version %d; apply migrations/001_init.sql through migrations/002_snapshot_frames.sql before starting PostgreSQL mode", m.Current, m.Required)
}

func NewPostgres(ctx context.Context, dsn string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Postgres{pool: pool}, nil
}

func (p *Postgres) Name() string {
	return "postgres"
}

func (p *Postgres) Ready(ctx context.Context) (model.StoreStatus, error) {
	if err := p.pool.Ping(ctx); err != nil {
		return model.StoreStatus{}, err
	}
	status, err := p.MigrationStatus(ctx)
	if err != nil {
		return model.StoreStatus{}, err
	}
	if err := status.Error(); err != nil {
		return model.StoreStatus{}, err
	}
	return model.StoreStatus{Store: p.Name(), MigrationVersion: status.Current}, nil
}

func (p *Postgres) MigrationStatus(ctx context.Context) (MigrationStatus, error) {
	version, err := p.migrationVersion(ctx)
	if err != nil {
		return MigrationStatus{}, err
	}
	return MigrationStatus{Current: version, Required: CurrentMigrationVersion}, nil
}

func (p *Postgres) RequireMigrations(ctx context.Context) error {
	status, err := p.MigrationStatus(ctx)
	if err != nil {
		return err
	}
	return status.Error()
}

func (p *Postgres) migrationVersion(ctx context.Context) (int, error) {
	var hasTable bool
	if err := p.pool.QueryRow(ctx, `
SELECT EXISTS (
	SELECT 1
	FROM information_schema.tables
	WHERE table_schema = current_schema()
		AND table_name = 'schema_migrations'
)`).Scan(&hasTable); err != nil {
		return 0, fmt.Errorf("migration status: %w", err)
	}
	if !hasTable {
		return 0, nil
	}
	var version int
	if err := p.pool.QueryRow(ctx, `
SELECT COALESCE(MAX(version),0)
FROM schema_migrations
WHERE name='ship_sim'`).Scan(&version); err != nil {
		return 0, fmt.Errorf("migration status: %w", err)
	}
	return version, nil
}

func (p *Postgres) SaveRun(ctx context.Context, run model.Run) error {
	if run.UpdatedAt.IsZero() {
		run.UpdatedAt = run.CreatedAt
	}
	if run.UpdatedAt.IsZero() {
		run.UpdatedAt = time.Now().UTC()
	}
	scenario, err := json.Marshal(run.Scenario)
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(ctx, `
INSERT INTO sim_runs (id, name, status, owner_id, scenario_name, scenario, created_at, updated_at, started_at, stopped_at, safety_notice)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
ON CONFLICT (id) DO UPDATE SET
	status = EXCLUDED.status,
	owner_id = EXCLUDED.owner_id,
	updated_at = EXCLUDED.updated_at,
	started_at = EXCLUDED.started_at,
	stopped_at = EXCLUDED.stopped_at,
	scenario = EXCLUDED.scenario`,
		run.ID, run.Name, run.Status, nullableString(run.OwnerID), run.Scenario.Name, scenario, run.CreatedAt, run.UpdatedAt,
		nullableTime(run.StartedAt), nullableTime(run.StoppedAt), run.SafetyNotice)
	if err != nil {
		return fmt.Errorf("save run: %w", err)
	}
	for _, sensor := range run.Scenario.Sensors {
		_, err = p.pool.Exec(ctx, `
INSERT INTO sensors (id, run_id, name, kind, position)
VALUES ($1,$2,$3,$4,ST_SetSRID(ST_MakePoint($5,$6,$7),4326))
ON CONFLICT (run_id, id) DO NOTHING`,
			sensor.ID, run.ID, sensor.Name, sensor.Kind, sensor.Position.Lon, sensor.Position.Lat, sensor.Position.Alt)
		if err != nil {
			return fmt.Errorf("save sensor: %w", err)
		}
	}
	for _, zone := range run.Scenario.Zones {
		geom, err := polygonWKT(zone.Polygon)
		if err != nil {
			return err
		}
		_, err = p.pool.Exec(ctx, `
INSERT INTO zones (id, run_id, name, kind, geom)
VALUES ($1,$2,$3,$4,ST_GeomFromText($5,4326))
ON CONFLICT (run_id, id) DO NOTHING`, zone.ID, run.ID, zone.Name, zone.Kind, geom)
		if err != nil {
			return fmt.Errorf("save zone: %w", err)
		}
	}
	return nil
}

func (p *Postgres) GetRun(ctx context.Context, id string) (model.Run, error) {
	var run model.Run
	var scenarioBytes []byte
	var startedAt *time.Time
	var stoppedAt *time.Time
	err := p.pool.QueryRow(ctx, `
SELECT id, name, status, COALESCE(owner_id,''), scenario, created_at, updated_at, started_at, stopped_at, safety_notice
FROM sim_runs WHERE id = $1`, id).Scan(
		&run.ID, &run.Name, &run.Status, &run.OwnerID, &scenarioBytes, &run.CreatedAt, &run.UpdatedAt, &startedAt, &stoppedAt, &run.SafetyNotice)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Run{}, errors.New("run not found")
	}
	if err != nil {
		return model.Run{}, err
	}
	if err := json.Unmarshal(scenarioBytes, &run.Scenario); err != nil {
		return model.Run{}, err
	}
	if startedAt != nil {
		run.StartedAt = *startedAt
	}
	if stoppedAt != nil {
		run.StoppedAt = *stoppedAt
	}
	return run, nil
}

func (p *Postgres) GetRunForOwner(ctx context.Context, id, ownerID string) (model.Run, error) {
	var run model.Run
	var scenarioBytes []byte
	var startedAt *time.Time
	var stoppedAt *time.Time
	err := p.pool.QueryRow(ctx, `
SELECT id, name, status, COALESCE(owner_id,''), scenario, created_at, updated_at, started_at, stopped_at, safety_notice
FROM sim_runs
WHERE id = $1 AND ($2 = '' OR owner_id = $2)`, id, ownerID).Scan(
		&run.ID, &run.Name, &run.Status, &run.OwnerID, &scenarioBytes, &run.CreatedAt, &run.UpdatedAt, &startedAt, &stoppedAt, &run.SafetyNotice)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Run{}, errors.New("run not found")
	}
	if err != nil {
		return model.Run{}, err
	}
	if err := json.Unmarshal(scenarioBytes, &run.Scenario); err != nil {
		return model.Run{}, err
	}
	if startedAt != nil {
		run.StartedAt = *startedAt
	}
	if stoppedAt != nil {
		run.StoppedAt = *stoppedAt
	}
	return run, nil
}

func (p *Postgres) ListRuns(ctx context.Context, limit int) ([]model.Run, error) {
	limit = normalizeLimit(limit, 50, 100)
	rows, err := p.pool.Query(ctx, `
SELECT id, name, status, COALESCE(owner_id,''), scenario, created_at, updated_at, started_at, stopped_at, safety_notice
FROM sim_runs ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var runs []model.Run
	for rows.Next() {
		var run model.Run
		var scenarioBytes []byte
		var startedAt *time.Time
		var stoppedAt *time.Time
		if err := rows.Scan(&run.ID, &run.Name, &run.Status, &run.OwnerID, &scenarioBytes, &run.CreatedAt, &run.UpdatedAt, &startedAt, &stoppedAt, &run.SafetyNotice); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(scenarioBytes, &run.Scenario); err != nil {
			return nil, err
		}
		if startedAt != nil {
			run.StartedAt = *startedAt
		}
		if stoppedAt != nil {
			run.StoppedAt = *stoppedAt
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (p *Postgres) ListRunsForOwner(ctx context.Context, limit int, ownerID string) ([]model.Run, error) {
	limit = normalizeLimit(limit, 50, 100)
	rows, err := p.pool.Query(ctx, `
SELECT id, name, status, COALESCE(owner_id,''), scenario, created_at, updated_at, started_at, stopped_at, safety_notice
FROM sim_runs
WHERE ($2 = '' OR owner_id = $2)
ORDER BY created_at DESC LIMIT $1`, limit, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var runs []model.Run
	for rows.Next() {
		var run model.Run
		var scenarioBytes []byte
		var startedAt *time.Time
		var stoppedAt *time.Time
		if err := rows.Scan(&run.ID, &run.Name, &run.Status, &run.OwnerID, &scenarioBytes, &run.CreatedAt, &run.UpdatedAt, &startedAt, &stoppedAt, &run.SafetyNotice); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(scenarioBytes, &run.Scenario); err != nil {
			return nil, err
		}
		if startedAt != nil {
			run.StartedAt = *startedAt
		}
		if stoppedAt != nil {
			run.StoppedAt = *stoppedAt
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (p *Postgres) SaveEvent(ctx context.Context, event model.SimEvent) error {
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(ctx, `
INSERT INTO sim_events (run_id, occurred_at, event_type, subject_id, payload)
VALUES ($1,$2,$3,$4,$5)`, event.RunID, event.OccurredAt, event.Type, event.SubjectID, payload)
	return err
}

func (p *Postgres) ListEvents(ctx context.Context, runID string, query model.EventQuery) (model.EventPage, error) {
	offset := 0
	if query.Cursor != "" {
		parsed, err := strconv.Atoi(query.Cursor)
		if err != nil || parsed < 0 {
			return model.EventPage{}, errors.New("invalid event cursor")
		}
		offset = parsed
	}
	limit := normalizeLimit(query.Limit, 50, 200)
	rows, err := p.pool.Query(ctx, `
SELECT id::text, run_id, occurred_at, event_type, COALESCE(subject_id,''), payload
FROM sim_events WHERE run_id=$1 ORDER BY occurred_at DESC, id DESC LIMIT $2 OFFSET $3`, runID, limit+1, offset)
	if err != nil {
		return model.EventPage{}, err
	}
	defer rows.Close()
	var events []model.SimEvent
	for rows.Next() {
		var event model.SimEvent
		var payload []byte
		if err := rows.Scan(&event.ID, &event.RunID, &event.OccurredAt, &event.Type, &event.SubjectID, &payload); err != nil {
			return model.EventPage{}, err
		}
		_ = json.Unmarshal(payload, &event.Payload)
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return model.EventPage{}, err
	}
	page := model.EventPage{Items: events}
	if len(events) > limit {
		page.Items = events[:limit]
		page.NextCursor = strconv.Itoa(offset + limit)
	}
	return page, nil
}

func (p *Postgres) SaveSnapshot(ctx context.Context, snapshot model.Snapshot) error {
	frame := snapshotFrame(snapshot)
	tracksJSON, err := json.Marshal(frame.Tracks)
	if err != nil {
		return err
	}
	contactsJSON, err := json.Marshal(frame.Contacts)
	if err != nil {
		return err
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if statementTimeout, ok := statementTimeoutFromContext(ctx); ok {
		if _, err = tx.Exec(ctx, `SELECT set_config('statement_timeout', $1, true)`, statementTimeout); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `
INSERT INTO sim_snapshots (run_id, tick, sampled_at, status, tracks, contacts, notice, snapshot_hz)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		frame.RunID, frame.Tick, frame.SampledAt, frame.Status, tracksJSON, contactsJSON, frame.Notice, frame.SnapshotHz)
	if err != nil {
		return err
	}
	var batch pgx.Batch
	for _, contact := range frame.Contacts {
		batch.Queue(`
INSERT INTO contacts_raw (run_id, sensor_id, contact_id, observed_at, position, velocity_x, velocity_y, velocity_z, confidence, kind)
VALUES ($1,$2,$3,$4,ST_SetSRID(ST_MakePoint($5,$6,$7),4326),$8,$9,$10,$11,$12)`,
			frame.RunID, contact.SensorID, contact.ID, contact.Timestamp,
			contact.Position.Lon, contact.Position.Lat, contact.Position.Alt,
			contact.Velocity.Lon, contact.Velocity.Lat, contact.Velocity.Alt,
			contact.Confidence, contact.Kind)
	}
	for _, track := range frame.Tracks {
		batch.Queue(`
INSERT INTO tracks (id, run_id, track_no, kind, threat_level, latest_position, confidence, status, last_seen_at)
VALUES ($1,$2,$3,$4,$5,ST_SetSRID(ST_MakePoint($6,$7,$8),4326),$9,$10,$11)
ON CONFLICT (id) DO UPDATE SET
	threat_level=EXCLUDED.threat_level,
	latest_position=EXCLUDED.latest_position,
	confidence=EXCLUDED.confidence,
	status=EXCLUDED.status,
	last_seen_at=EXCLUDED.last_seen_at`,
			track.ID, frame.RunID, track.TrackNo, track.Kind, track.Threat,
			track.Position.Lon, track.Position.Lat, track.Position.Alt,
			track.Confidence, track.Status, track.UpdatedAt)
		batch.Queue(`
INSERT INTO track_points (run_id, track_id, sampled_at, position, speed, heading, confidence)
VALUES ($1,$2,$3,ST_SetSRID(ST_MakePoint($4,$5,$6),4326),$7,$8,$9)`,
			frame.RunID, track.ID, track.UpdatedAt, track.Position.Lon, track.Position.Lat, track.Position.Alt,
			abs(track.Velocity.Lon)+abs(track.Velocity.Lat), 0.0, track.Confidence)
	}
	if batch.Len() > 0 {
		results := tx.SendBatch(ctx, &batch)
		if err := results.Close(); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (p *Postgres) ListSnapshots(ctx context.Context, runID string, query model.SnapshotQuery) ([]model.SnapshotFrame, error) {
	limit := normalizeLimit(query.Limit, 200, 1000)
	rows, err := p.pool.Query(ctx, `
SELECT run_id::text, status, tick, sampled_at, tracks, contacts, notice, snapshot_hz
FROM sim_snapshots
WHERE run_id=$1
	AND ($2::timestamptz IS NULL OR sampled_at >= $2)
	AND ($3::timestamptz IS NULL OR sampled_at <= $3)
ORDER BY sampled_at, id
LIMIT $4`, runID, nullableTime(query.From), nullableTime(query.To), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var frames []model.SnapshotFrame
	for rows.Next() {
		frame, err := scanSnapshotFrame(rows.Scan)
		if err != nil {
			return nil, err
		}
		frames = append(frames, frame)
	}
	return frames, rows.Err()
}

func (p *Postgres) NearestSnapshot(ctx context.Context, runID string, at time.Time) (model.SnapshotFrame, error) {
	if at.IsZero() {
		return p.LatestSnapshot(ctx, runID)
	}
	frame, err := scanSnapshotFrame(p.pool.QueryRow(ctx, `
SELECT run_id::text, status, tick, sampled_at, tracks, contacts, notice, snapshot_hz
FROM sim_snapshots
WHERE run_id=$1
ORDER BY ABS(EXTRACT(EPOCH FROM sampled_at - $2::timestamptz)), sampled_at DESC, id DESC
LIMIT 1`, runID, at).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.SnapshotFrame{}, errors.New("snapshot not found")
	}
	return frame, err
}

func (p *Postgres) LatestSnapshot(ctx context.Context, runID string) (model.SnapshotFrame, error) {
	frame, err := scanSnapshotFrame(p.pool.QueryRow(ctx, `
SELECT run_id::text, status, tick, sampled_at, tracks, contacts, notice, snapshot_hz
FROM sim_snapshots
WHERE run_id=$1
ORDER BY sampled_at DESC, id DESC
LIMIT 1`, runID).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.SnapshotFrame{}, errors.New("snapshot not found")
	}
	return frame, err
}

func (p *Postgres) SnapshotRange(ctx context.Context, runID string) (model.SnapshotRange, bool, error) {
	var from *time.Time
	var to *time.Time
	var count int64
	err := p.pool.QueryRow(ctx, `
SELECT MIN(sampled_at), MAX(sampled_at), COUNT(*)
FROM sim_snapshots
WHERE run_id=$1`, runID).Scan(&from, &to, &count)
	if err != nil {
		return model.SnapshotRange{}, false, err
	}
	if count == 0 || from == nil || to == nil {
		return model.SnapshotRange{}, false, nil
	}
	return model.SnapshotRange{From: *from, To: *to, Count: int(count)}, true, nil
}

func (p *Postgres) ListTracks(ctx context.Context, runID string) ([]model.Track, error) {
	rows, err := p.pool.Query(ctx, `
SELECT id, track_no, kind, threat_level, ST_X(latest_position), ST_Y(latest_position), COALESCE(ST_Z(latest_position),0), confidence, status, last_seen_at
FROM tracks WHERE run_id=$1 ORDER BY track_no`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tracks []model.Track
	for rows.Next() {
		var track model.Track
		if err := rows.Scan(&track.ID, &track.TrackNo, &track.Kind, &track.Threat,
			&track.Position.Lon, &track.Position.Lat, &track.Position.Alt,
			&track.Confidence, &track.Status, &track.UpdatedAt); err != nil {
			return nil, err
		}
		tracks = append(tracks, track)
	}
	return tracks, rows.Err()
}

func (p *Postgres) ListTrackPoints(ctx context.Context, runID string, query model.TrackPointQuery) ([]model.TrackPoint, error) {
	limit := normalizeLimit(query.Limit, 200, 1000)
	rows, err := p.pool.Query(ctx, `
SELECT track_id::text, sampled_at, ST_X(position), ST_Y(position), COALESCE(ST_Z(position),0), COALESCE(speed,0), COALESCE(heading,0), COALESCE(confidence,0)
FROM track_points
WHERE run_id=$1
	AND ($2 = '' OR track_id::text = $2)
	AND ($3::timestamptz IS NULL OR sampled_at >= $3)
	AND ($4::timestamptz IS NULL OR sampled_at <= $4)
ORDER BY sampled_at, id
LIMIT $5`, runID, query.TrackID, nullableTime(query.From), nullableTime(query.To), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var points []model.TrackPoint
	for rows.Next() {
		var point model.TrackPoint
		if err := rows.Scan(&point.TrackID, &point.SampledAt,
			&point.Position.Lon, &point.Position.Lat, &point.Position.Alt,
			&point.Speed, &point.Heading, &point.Confidence); err != nil {
			return nil, err
		}
		points = append(points, point)
	}
	return points, rows.Err()
}

func (p *Postgres) ListZones(ctx context.Context, runID string) ([]model.Zone, error) {
	rows, err := p.pool.Query(ctx, `
SELECT id, name, kind, ST_AsGeoJSON(geom) FROM zones WHERE run_id=$1 ORDER BY name`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var zones []model.Zone
	for rows.Next() {
		var zone model.Zone
		var geojson string
		if err := rows.Scan(&zone.ID, &zone.Name, &zone.Kind, &geojson); err != nil {
			return nil, err
		}
		zone.Polygon = polygonFromGeoJSON(geojson)
		zones = append(zones, zone)
	}
	return zones, rows.Err()
}

func (p *Postgres) ListScenarioSummaries(ctx context.Context) ([]model.ScenarioSummary, error) {
	rows, err := p.pool.Query(ctx, `
SELECT id::text, name, version, body
FROM scenarios
ORDER BY name, version DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var summaries []model.ScenarioSummary
	for rows.Next() {
		var summary model.ScenarioSummary
		var body []byte
		if err := rows.Scan(&summary.ID, &summary.Name, &summary.Version, &body); err != nil {
			return nil, err
		}
		var scenario model.Scenario
		_ = json.Unmarshal(body, &scenario)
		if scenario.Description != "" {
			summary.Description = scenario.Description
		}
		summary.Source = "database"
		summaries = append(summaries, summary)
	}
	return summaries, rows.Err()
}

func (p *Postgres) GetScenario(ctx context.Context, id string) (model.Scenario, error) {
	var scenario model.Scenario
	var body []byte
	var name string
	var version int
	err := p.pool.QueryRow(ctx, `
SELECT name, version, body
FROM scenarios
WHERE id::text=$1`, id).Scan(&name, &version, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Scenario{}, errors.New("scenario not found")
	}
	if err != nil {
		return model.Scenario{}, err
	}
	if err := json.Unmarshal(body, &scenario); err != nil {
		return model.Scenario{}, err
	}
	if scenario.ID == "" {
		scenario.ID = id
	}
	if scenario.Name == "" {
		scenario.Name = name
	}
	if scenario.Version == 0 {
		scenario.Version = version
	}
	return scenario, nil
}

func (p *Postgres) PreviewPrune(ctx context.Context, policy model.RetentionPolicy) (model.RetentionPreview, error) {
	var preview model.RetentionPreview
	if !policy.EndedBefore.IsZero() {
		if err := p.pool.QueryRow(ctx, `
SELECT COUNT(*)
FROM sim_runs
WHERE stopped_at IS NOT NULL
	AND stopped_at < $1
	AND ($2 = '' OR owner_id = $2)`, policy.EndedBefore, policy.OwnerID).Scan(&preview.RunsMatched); err != nil {
			return preview, err
		}
	}
	if err := p.pool.QueryRow(ctx, `
SELECT COUNT(*)
FROM sim_events e
LEFT JOIN sim_runs r ON r.id=e.run_id
WHERE (
		($1::timestamptz IS NOT NULL AND r.stopped_at IS NOT NULL AND r.stopped_at < $1)
		OR ($2::timestamptz IS NOT NULL AND e.occurred_at < $2)
	)
	AND ($3 = '' OR r.owner_id = $3)`,
		nullableTime(policy.EndedBefore), nullableTime(policy.Cutoff), policy.OwnerID).Scan(&preview.EventsMatched); err != nil {
		return preview, err
	}
	if err := p.pool.QueryRow(ctx, `
SELECT COUNT(*)
FROM contacts_raw c
LEFT JOIN sim_runs r ON r.id=c.run_id
WHERE (
		($1::timestamptz IS NOT NULL AND r.stopped_at IS NOT NULL AND r.stopped_at < $1)
		OR ($2::timestamptz IS NOT NULL AND c.observed_at < $2)
	)
	AND ($3 = '' OR r.owner_id = $3)`,
		nullableTime(policy.EndedBefore), nullableTime(policy.Cutoff), policy.OwnerID).Scan(&preview.ContactsMatched); err != nil {
		return preview, err
	}
	if err := p.pool.QueryRow(ctx, `
SELECT COUNT(*)
FROM track_points p
LEFT JOIN sim_runs r ON r.id=p.run_id
WHERE (
		($1::timestamptz IS NOT NULL AND r.stopped_at IS NOT NULL AND r.stopped_at < $1)
		OR ($2::timestamptz IS NOT NULL AND p.sampled_at < $2)
	)
	AND ($3 = '' OR r.owner_id = $3)`,
		nullableTime(policy.EndedBefore), nullableTime(policy.Cutoff), policy.OwnerID).Scan(&preview.TrackPointsMatched); err != nil {
		return preview, err
	}
	if err := p.pool.QueryRow(ctx, `
SELECT COUNT(*)
FROM sim_snapshots s
LEFT JOIN sim_runs r ON r.id=s.run_id
WHERE (
		($1::timestamptz IS NOT NULL AND r.stopped_at IS NOT NULL AND r.stopped_at < $1)
		OR ($2::timestamptz IS NOT NULL AND s.sampled_at < $2)
	)
	AND ($3 = '' OR r.owner_id = $3)`,
		nullableTime(policy.EndedBefore), nullableTime(policy.Cutoff), policy.OwnerID).Scan(&preview.SnapshotsMatched); err != nil {
		return preview, err
	}
	if policy.MaxTrackPointsPerRun > 0 {
		var excess int64
		if err := p.pool.QueryRow(ctx, `
WITH ranked AS (
	SELECT row_number() OVER (PARTITION BY p.run_id ORDER BY p.sampled_at DESC, p.id DESC) AS rn
	FROM track_points p
	JOIN sim_runs r ON r.id=p.run_id
	WHERE ($2 = '' OR r.owner_id = $2)
)
SELECT COUNT(*) FROM ranked WHERE rn > $1`, policy.MaxTrackPointsPerRun, policy.OwnerID).Scan(&excess); err != nil {
			return preview, err
		}
		preview.TrackPointsMatched += excess
	}
	return preview, nil
}

func (p *Postgres) Prune(ctx context.Context, policy model.RetentionPolicy) (model.RetentionResult, error) {
	var result model.RetentionResult
	if !policy.EndedBefore.IsZero() {
		if err := p.pool.QueryRow(ctx, `
SELECT COUNT(*)
FROM sim_runs
WHERE stopped_at IS NOT NULL
	AND stopped_at < $1
	AND ($2 = '' OR owner_id = $2)`, policy.EndedBefore, policy.OwnerID).Scan(&result.RunsMatched); err != nil {
			return result, err
		}
	}
	if !policy.Cutoff.IsZero() || !policy.EndedBefore.IsZero() {
		tag, err := p.pool.Exec(ctx, `
DELETE FROM sim_events e
USING sim_runs r
WHERE e.run_id=r.id
	AND (
		($1::timestamptz IS NOT NULL AND r.stopped_at IS NOT NULL AND r.stopped_at < $1)
		OR ($2::timestamptz IS NOT NULL AND e.occurred_at < $2)
	)
	AND ($3 = '' OR r.owner_id = $3)`, nullableTime(policy.EndedBefore), nullableTime(policy.Cutoff), policy.OwnerID)
		if err != nil {
			return result, err
		}
		result.EventsDeleted += tag.RowsAffected()
		tag, err = p.pool.Exec(ctx, `
DELETE FROM contacts_raw c
USING sim_runs r
WHERE c.run_id=r.id
	AND (
		($1::timestamptz IS NOT NULL AND r.stopped_at IS NOT NULL AND r.stopped_at < $1)
		OR ($2::timestamptz IS NOT NULL AND c.observed_at < $2)
	)
	AND ($3 = '' OR r.owner_id = $3)`, nullableTime(policy.EndedBefore), nullableTime(policy.Cutoff), policy.OwnerID)
		if err != nil {
			return result, err
		}
		result.ContactsDeleted += tag.RowsAffected()
		tag, err = p.pool.Exec(ctx, `
DELETE FROM track_points p
USING sim_runs r
WHERE p.run_id=r.id
	AND (
		($1::timestamptz IS NOT NULL AND r.stopped_at IS NOT NULL AND r.stopped_at < $1)
		OR ($2::timestamptz IS NOT NULL AND p.sampled_at < $2)
	)
	AND ($3 = '' OR r.owner_id = $3)`, nullableTime(policy.EndedBefore), nullableTime(policy.Cutoff), policy.OwnerID)
		if err != nil {
			return result, err
		}
		result.TrackPointsDeleted += tag.RowsAffected()
		tag, err = p.pool.Exec(ctx, `
DELETE FROM sim_snapshots s
USING sim_runs r
WHERE s.run_id=r.id
	AND (
		($1::timestamptz IS NOT NULL AND r.stopped_at IS NOT NULL AND r.stopped_at < $1)
		OR ($2::timestamptz IS NOT NULL AND s.sampled_at < $2)
	)
	AND ($3 = '' OR r.owner_id = $3)`, nullableTime(policy.EndedBefore), nullableTime(policy.Cutoff), policy.OwnerID)
		if err != nil {
			return result, err
		}
		result.SnapshotsDeleted += tag.RowsAffected()
	}
	if policy.MaxTrackPointsPerRun > 0 {
		tag, err := p.pool.Exec(ctx, `
WITH ranked AS (
	SELECT p.id,
		row_number() OVER (PARTITION BY p.run_id ORDER BY p.sampled_at DESC, p.id DESC) AS rn
	FROM track_points p
	JOIN sim_runs r ON r.id=p.run_id
	WHERE ($2 = '' OR r.owner_id = $2)
)
DELETE FROM track_points
USING ranked
WHERE track_points.id = ranked.id
	AND ranked.rn > $1`, policy.MaxTrackPointsPerRun, policy.OwnerID)
		if err != nil {
			return result, err
		}
		result.TrackPointsDeleted += tag.RowsAffected()
	}
	return result, nil
}

func (p *Postgres) Close() {
	p.pool.Close()
}

func nullableTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func nullableString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func statementTimeoutFromContext(ctx context.Context) (string, bool) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return "", false
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		remaining = time.Millisecond
	}
	ms := remaining.Milliseconds()
	if ms < 1 {
		ms = 1
	}
	return fmt.Sprintf("%dms", ms), true
}

func scanSnapshotFrame(scan func(dest ...any) error) (model.SnapshotFrame, error) {
	var frame model.SnapshotFrame
	var tracksJSON []byte
	var contactsJSON []byte
	if err := scan(&frame.RunID, &frame.Status, &frame.Tick, &frame.SampledAt, &tracksJSON, &contactsJSON, &frame.Notice, &frame.SnapshotHz); err != nil {
		return model.SnapshotFrame{}, err
	}
	if err := json.Unmarshal(tracksJSON, &frame.Tracks); err != nil {
		return model.SnapshotFrame{}, err
	}
	if err := json.Unmarshal(contactsJSON, &frame.Contacts); err != nil {
		return model.SnapshotFrame{}, err
	}
	if frame.Tracks == nil {
		frame.Tracks = []model.Track{}
	}
	if frame.Contacts == nil {
		frame.Contacts = []model.Contact{}
	}
	return frame, nil
}

func polygonWKT(points []model.Vec3) (string, error) {
	if len(points) < 3 {
		return "", errors.New("zone polygon requires at least 3 points")
	}
	wkt := "POLYGON(("
	for i, point := range points {
		if i > 0 {
			wkt += ","
		}
		wkt += fmt.Sprintf("%f %f", point.Lon, point.Lat)
	}
	first := points[0]
	wkt += fmt.Sprintf(",%f %f))", first.Lon, first.Lat)
	return wkt, nil
}

func polygonFromGeoJSON(input string) []model.Vec3 {
	var geo struct {
		Type        string        `json:"type"`
		Coordinates [][][]float64 `json:"coordinates"`
	}
	if err := json.Unmarshal([]byte(input), &geo); err != nil || strings.ToLower(geo.Type) != "polygon" || len(geo.Coordinates) == 0 {
		return nil
	}
	ring := geo.Coordinates[0]
	out := make([]model.Vec3, 0, len(ring))
	for _, point := range ring {
		if len(point) < 2 {
			continue
		}
		vec := model.Vec3{Lon: point[0], Lat: point[1]}
		if len(point) > 2 {
			vec.Alt = point[2]
		}
		out = append(out, vec)
	}
	if len(out) > 1 && out[0] == out[len(out)-1] {
		out = out[:len(out)-1]
	}
	return out
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
