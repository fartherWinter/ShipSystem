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

const CurrentMigrationVersion = 3

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
	return fmt.Errorf("database migrations are not current: current version %d, required version %d; apply migrations/001_init.sql through migrations/003_training_product.sql before starting PostgreSQL mode", m.Current, m.Required)
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
	tags, err := json.Marshal(run.Tags)
	if err != nil {
		return err
	}
	trainees, err := json.Marshal(run.Trainees)
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(ctx, `
INSERT INTO sim_runs (id, name, status, owner_id, scenario_name, scenario, created_at, updated_at, started_at, stopped_at, safety_notice, tags, trainees, instructor_notes, archived_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
ON CONFLICT (id) DO UPDATE SET
	name = EXCLUDED.name,
	status = EXCLUDED.status,
	owner_id = EXCLUDED.owner_id,
	updated_at = EXCLUDED.updated_at,
	started_at = EXCLUDED.started_at,
	stopped_at = EXCLUDED.stopped_at,
	scenario = EXCLUDED.scenario,
	tags = EXCLUDED.tags,
	trainees = EXCLUDED.trainees,
	instructor_notes = EXCLUDED.instructor_notes,
	archived_at = EXCLUDED.archived_at,
	safety_notice = EXCLUDED.safety_notice`,
		run.ID, run.Name, run.Status, nullableString(run.OwnerID), run.Scenario.Name, scenario, run.CreatedAt, run.UpdatedAt,
		nullableTime(run.StartedAt), nullableTime(run.StoppedAt), run.SafetyNotice, tags, trainees, run.InstructorNotes, nullableTime(run.ArchivedAt))
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
	run, err := scanRun(p.pool.QueryRow(ctx, `
SELECT id, name, status, COALESCE(owner_id,''), scenario, created_at, updated_at, started_at, stopped_at, archived_at, safety_notice, tags, trainees, instructor_notes
FROM sim_runs WHERE id = $1`, id).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Run{}, errors.New("run not found")
	}
	if err != nil {
		return model.Run{}, err
	}
	return run, nil
}

func (p *Postgres) GetRunForOwner(ctx context.Context, id, ownerID string) (model.Run, error) {
	run, err := scanRun(p.pool.QueryRow(ctx, `
SELECT id, name, status, COALESCE(owner_id,''), scenario, created_at, updated_at, started_at, stopped_at, archived_at, safety_notice, tags, trainees, instructor_notes
FROM sim_runs
WHERE id = $1 AND ($2 = '' OR owner_id = $2)`, id, ownerID).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Run{}, errors.New("run not found")
	}
	if err != nil {
		return model.Run{}, err
	}
	return run, nil
}

func (p *Postgres) ListRuns(ctx context.Context, limit int) ([]model.Run, error) {
	limit = normalizeLimit(limit, 50, 100)
	rows, err := p.pool.Query(ctx, `
SELECT id, name, status, COALESCE(owner_id,''), scenario, created_at, updated_at, started_at, stopped_at, archived_at, safety_notice, tags, trainees, instructor_notes
FROM sim_runs ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var runs []model.Run
	for rows.Next() {
		run, err := scanRun(rows.Scan)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (p *Postgres) ListRunsForOwner(ctx context.Context, limit int, ownerID string) ([]model.Run, error) {
	limit = normalizeLimit(limit, 50, 100)
	rows, err := p.pool.Query(ctx, `
SELECT id, name, status, COALESCE(owner_id,''), scenario, created_at, updated_at, started_at, stopped_at, archived_at, safety_notice, tags, trainees, instructor_notes
FROM sim_runs
WHERE ($2 = '' OR owner_id = $2)
ORDER BY created_at DESC LIMIT $1`, limit, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var runs []model.Run
	for rows.Next() {
		run, err := scanRun(rows.Scan)
		if err != nil {
			return nil, err
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
SELECT id::text, name, version, description, source, enabled, COALESCE(created_by,''), created_at, updated_at
FROM scenarios
ORDER BY name, version DESC, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var summaries []model.ScenarioSummary
	for rows.Next() {
		var summary model.ScenarioSummary
		if err := rows.Scan(&summary.ID, &summary.Name, &summary.Version, &summary.Description, &summary.Source, &summary.Enabled, &summary.CreatedBy, &summary.CreatedAt, &summary.UpdatedAt); err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	return summaries, rows.Err()
}

func (p *Postgres) GetScenario(ctx context.Context, id string) (model.Scenario, error) {
	record, err := p.GetScenarioRecord(ctx, id)
	if err != nil {
		return model.Scenario{}, err
	}
	return record.Scenario, nil
}

func (p *Postgres) GetScenarioRecord(ctx context.Context, id string) (model.ScenarioRecord, error) {
	var record model.ScenarioRecord
	var body []byte
	var name string
	var version int
	var description string
	err := p.pool.QueryRow(ctx, `
SELECT id::text, name, version, description, source, enabled, COALESCE(created_by,''), created_at, updated_at, body
FROM scenarios
WHERE id::text=$1`, id).Scan(
		&record.ID, &name, &version, &description, &record.Source,
		&record.Enabled, &record.CreatedBy, &record.CreatedAt, &record.UpdatedAt, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ScenarioRecord{}, errors.New("scenario not found")
	}
	if err != nil {
		return model.ScenarioRecord{}, err
	}
	if err := json.Unmarshal(body, &record.Scenario); err != nil {
		return model.ScenarioRecord{}, err
	}
	if record.Scenario.ID == "" {
		record.Scenario.ID = record.ID
	}
	if record.Scenario.Name == "" {
		record.Scenario.Name = name
	}
	if record.Scenario.Version == 0 {
		record.Scenario.Version = version
	}
	if record.Scenario.Description == "" {
		record.Scenario.Description = description
	}
	return record, nil
}

func (p *Postgres) SaveScenario(ctx context.Context, record model.ScenarioRecord) (model.ScenarioSummary, error) {
	if record.Source == "" {
		record.Source = "database"
	}
	if !record.Enabled && record.UpdatedAt.IsZero() {
		record.Enabled = true
	}
	body, err := json.Marshal(record.Scenario)
	if err != nil {
		return model.ScenarioSummary{}, err
	}
	var summary model.ScenarioSummary
	if record.ID == "" {
		err = p.pool.QueryRow(ctx, `
INSERT INTO scenarios (name, version, description, source, enabled, created_by, body)
VALUES ($1,$2,$3,$4,$5,$6,$7)
RETURNING id::text, name, version, description, source, enabled, COALESCE(created_by,''), created_at, updated_at`,
			record.Scenario.Name, record.Scenario.Version, record.Scenario.Description, record.Source, record.Enabled, nullableString(record.CreatedBy), body).Scan(
			&summary.ID, &summary.Name, &summary.Version, &summary.Description, &summary.Source, &summary.Enabled, &summary.CreatedBy, &summary.CreatedAt, &summary.UpdatedAt)
	} else {
		err = p.pool.QueryRow(ctx, `
UPDATE scenarios
SET name=$2, version=$3, description=$4, source=$5, enabled=$6, created_by=COALESCE(created_by,$7), body=$8, updated_at=now()
WHERE id::text=$1
RETURNING id::text, name, version, description, source, enabled, COALESCE(created_by,''), created_at, updated_at`,
			record.ID, record.Scenario.Name, record.Scenario.Version, record.Scenario.Description, record.Source, record.Enabled, nullableString(record.CreatedBy), body).Scan(
			&summary.ID, &summary.Name, &summary.Version, &summary.Description, &summary.Source, &summary.Enabled, &summary.CreatedBy, &summary.CreatedAt, &summary.UpdatedAt)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ScenarioSummary{}, errors.New("scenario not found")
	}
	if err != nil {
		return model.ScenarioSummary{}, err
	}
	if summary.ID != "" && record.Scenario.ID != summary.ID {
		updated := record.Scenario
		updated.ID = summary.ID
		updatedBody, marshalErr := json.Marshal(updated)
		if marshalErr != nil {
			return model.ScenarioSummary{}, marshalErr
		}
		if _, updateErr := p.pool.Exec(ctx, `UPDATE scenarios SET body=$2 WHERE id::text=$1`, summary.ID, updatedBody); updateErr != nil {
			return model.ScenarioSummary{}, updateErr
		}
	}
	return summary, nil
}

func (p *Postgres) SetScenarioEnabled(ctx context.Context, id string, enabled bool, actorID string) (model.ScenarioSummary, error) {
	var summary model.ScenarioSummary
	err := p.pool.QueryRow(ctx, `
UPDATE scenarios
SET enabled=$2, created_by=COALESCE(created_by,$3), updated_at=now()
WHERE id::text=$1
RETURNING id::text, name, version, description, source, enabled, COALESCE(created_by,''), created_at, updated_at`,
		id, enabled, nullableString(actorID)).Scan(
		&summary.ID, &summary.Name, &summary.Version, &summary.Description, &summary.Source, &summary.Enabled, &summary.CreatedBy, &summary.CreatedAt, &summary.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ScenarioSummary{}, errors.New("scenario not found")
	}
	return summary, err
}

func (p *Postgres) SaveEventAnnotation(ctx context.Context, annotation model.EventAnnotation) (model.EventAnnotation, error) {
	err := p.pool.QueryRow(ctx, `
INSERT INTO event_annotations (run_id, event_id, note, actor_id)
VALUES ($1,$2,$3,$4)
RETURNING id::text, run_id::text, COALESCE(event_id,''), note, COALESCE(actor_id,''), created_at`,
		annotation.RunID, nullableString(annotation.EventID), annotation.Note, nullableString(annotation.ActorID)).Scan(
		&annotation.ID, &annotation.RunID, &annotation.EventID, &annotation.Note, &annotation.ActorID, &annotation.CreatedAt)
	if err != nil {
		return model.EventAnnotation{}, err
	}
	return annotation, nil
}

func (p *Postgres) ListEventAnnotations(ctx context.Context, runID string) ([]model.EventAnnotation, error) {
	rows, err := p.pool.Query(ctx, `
SELECT id::text, run_id::text, COALESCE(event_id,''), note, COALESCE(actor_id,''), created_at
FROM event_annotations
WHERE run_id=$1
ORDER BY created_at, id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var annotations []model.EventAnnotation
	for rows.Next() {
		var annotation model.EventAnnotation
		if err := rows.Scan(&annotation.ID, &annotation.RunID, &annotation.EventID, &annotation.Note, &annotation.ActorID, &annotation.CreatedAt); err != nil {
			return nil, err
		}
		annotations = append(annotations, annotation)
	}
	return annotations, rows.Err()
}

func (p *Postgres) SaveAuditLog(ctx context.Context, log model.AuditLog) error {
	payload, err := json.Marshal(log.Payload)
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(ctx, `
INSERT INTO audit_logs (run_id, scenario_id, actor_id, action, target_type, target_id, occurred_at, payload)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		nullableString(log.RunID), nullableString(log.ScenarioID), nullableString(log.ActorID),
		log.Action, log.TargetType, log.TargetID, coalesceTime(log.OccurredAt), payload)
	return err
}

func (p *Postgres) ListAuditLogs(ctx context.Context, query model.AuditLogQuery) ([]model.AuditLog, error) {
	limit := normalizeLimit(query.Limit, 50, 200)
	rows, err := p.pool.Query(ctx, `
SELECT id::text, COALESCE(run_id::text,''), COALESCE(scenario_id,''), COALESCE(actor_id,''), action, target_type, target_id, occurred_at, payload
FROM audit_logs
WHERE ($1 = '' OR run_id::text = $1)
	AND ($2 = '' OR scenario_id = $2)
ORDER BY occurred_at DESC, id DESC
LIMIT $3`, query.RunID, query.ScenarioID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var logs []model.AuditLog
	for rows.Next() {
		var log model.AuditLog
		var payload []byte
		if err := rows.Scan(&log.ID, &log.RunID, &log.ScenarioID, &log.ActorID, &log.Action, &log.TargetType, &log.TargetID, &log.OccurredAt, &payload); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(payload, &log.Payload)
		logs = append(logs, log)
	}
	return logs, rows.Err()
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
	if policy.MaxEventsPerRun > 0 {
		var excess int64
		if err := p.pool.QueryRow(ctx, `
WITH ranked AS (
	SELECT row_number() OVER (PARTITION BY e.run_id ORDER BY e.occurred_at DESC, e.id DESC) AS rn
	FROM sim_events e
	JOIN sim_runs r ON r.id=e.run_id
	WHERE ($2 = '' OR r.owner_id = $2)
)
SELECT COUNT(*) FROM ranked WHERE rn > $1`, policy.MaxEventsPerRun, policy.OwnerID).Scan(&excess); err != nil {
			return preview, err
		}
		preview.EventsMatched += excess
	}
	if policy.MaxSnapshotsPerRun > 0 {
		var excess int64
		if err := p.pool.QueryRow(ctx, `
WITH ranked AS (
	SELECT row_number() OVER (PARTITION BY s.run_id ORDER BY s.sampled_at DESC, s.id DESC) AS rn
	FROM sim_snapshots s
	JOIN sim_runs r ON r.id=s.run_id
	WHERE ($2 = '' OR r.owner_id = $2)
)
SELECT COUNT(*) FROM ranked WHERE rn > $1`, policy.MaxSnapshotsPerRun, policy.OwnerID).Scan(&excess); err != nil {
			return preview, err
		}
		preview.SnapshotsMatched += excess
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
	if policy.MaxEventsPerRun > 0 {
		tag, err := p.pool.Exec(ctx, `
WITH ranked AS (
	SELECT e.id,
		row_number() OVER (PARTITION BY e.run_id ORDER BY e.occurred_at DESC, e.id DESC) AS rn
	FROM sim_events e
	JOIN sim_runs r ON r.id=e.run_id
	WHERE ($2 = '' OR r.owner_id = $2)
)
DELETE FROM sim_events
USING ranked
WHERE sim_events.id = ranked.id
	AND ranked.rn > $1`, policy.MaxEventsPerRun, policy.OwnerID)
		if err != nil {
			return result, err
		}
		result.EventsDeleted += tag.RowsAffected()
	}
	if policy.MaxSnapshotsPerRun > 0 {
		tag, err := p.pool.Exec(ctx, `
WITH ranked AS (
	SELECT s.id,
		row_number() OVER (PARTITION BY s.run_id ORDER BY s.sampled_at DESC, s.id DESC) AS rn
	FROM sim_snapshots s
	JOIN sim_runs r ON r.id=s.run_id
	WHERE ($2 = '' OR r.owner_id = $2)
)
DELETE FROM sim_snapshots
USING ranked
WHERE sim_snapshots.id = ranked.id
	AND ranked.rn > $1`, policy.MaxSnapshotsPerRun, policy.OwnerID)
		if err != nil {
			return result, err
		}
		result.SnapshotsDeleted += tag.RowsAffected()
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

func coalesceTime(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now().UTC()
	}
	return t
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

func scanRun(scan func(dest ...any) error) (model.Run, error) {
	var run model.Run
	var scenarioBytes []byte
	var tagsJSON []byte
	var traineesJSON []byte
	var startedAt *time.Time
	var stoppedAt *time.Time
	var archivedAt *time.Time
	if err := scan(&run.ID, &run.Name, &run.Status, &run.OwnerID, &scenarioBytes, &run.CreatedAt, &run.UpdatedAt, &startedAt, &stoppedAt, &archivedAt, &run.SafetyNotice, &tagsJSON, &traineesJSON, &run.InstructorNotes); err != nil {
		return model.Run{}, err
	}
	if err := json.Unmarshal(scenarioBytes, &run.Scenario); err != nil {
		return model.Run{}, err
	}
	if len(tagsJSON) > 0 {
		if err := json.Unmarshal(tagsJSON, &run.Tags); err != nil {
			return model.Run{}, err
		}
	}
	if len(traineesJSON) > 0 {
		if err := json.Unmarshal(traineesJSON, &run.Trainees); err != nil {
			return model.Run{}, err
		}
	}
	if startedAt != nil {
		run.StartedAt = *startedAt
	}
	if stoppedAt != nil {
		run.StoppedAt = *stoppedAt
	}
	if archivedAt != nil {
		run.ArchivedAt = *archivedAt
	}
	return run, nil
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
