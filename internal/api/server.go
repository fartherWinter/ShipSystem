package api

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"shipsim/internal/config"
	"shipsim/internal/model"
	"shipsim/internal/sim"
)

type Server struct {
	manager                *sim.Manager
	logger                 *slog.Logger
	cfg                    config.Config
	upgrader               websocket.Upgrader
	allowedOrigins         map[string]struct{}
	staticHandler          http.Handler
	staticDir              string
	wsTickets              map[string]wsTicket
	wsTicketsMu            sync.Mutex
	wsConnections          atomic.Int64
	requestCount           atomic.Int64
	requestErrors          atomic.Int64
	requestDurationTotalNS atomic.Int64
	requestDurationMaxNS   atomic.Int64
}

type wsTicket struct {
	RunID     string
	UserID    string
	ExpiresAt time.Time
}

type wsTicketResponse struct {
	Ticket    string    `json:"ticket"`
	ExpiresAt time.Time `json:"expires_at"`
}

type metricsSnapshot struct {
	ActiveRuns           int
	ListedRuns           int
	WebSocketConnections int64
	SnapshotFrames       int
	SnapshotFramesByRun  map[string]int
	Runtime              sim.RuntimeMetrics
	RequestCount         int64
	RequestErrors        int64
	RequestDurationAvgMS float64
	RequestDurationMaxMS float64
	EngineCount          int
	RunningEngineCount   int
	StoreReady           bool
	StoreStatus          model.StoreStatus
	StoreError           string
	SampleLimit          int
}

const wsTicketTTL = 30 * time.Second

func NewServer(manager *sim.Manager, logger *slog.Logger) *Server {
	return NewServerWithConfig(manager, logger, config.Default())
}

func NewServerWithConfig(manager *sim.Manager, logger *slog.Logger, cfg config.Config) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	server := &Server{
		manager:        manager,
		logger:         logger,
		cfg:            cfg,
		allowedOrigins: parseAllowedOrigins(cfg.AllowedOrigins),
		wsTickets:      map[string]wsTicket{},
	}
	if cfg.StaticDir != "" {
		server.staticDir = cfg.StaticDir
		server.staticHandler = http.FileServer(http.Dir(cfg.StaticDir))
	}
	server.upgrader = websocket.Upgrader{
		CheckOrigin: server.checkOrigin,
	}
	return server
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.health)
	mux.HandleFunc("/readyz", s.ready)
	mux.HandleFunc("/metrics", s.metrics)
	mux.HandleFunc("/metrics/prometheus", s.prometheusMetrics)
	mux.HandleFunc("/api/retention/preview", s.retentionPreview)
	mux.HandleFunc("/api/retention/prune", s.retentionPrune)
	mux.HandleFunc("/api/runs", s.runs)
	mux.HandleFunc("/api/runs/", s.run)
	mux.HandleFunc("/api/scenarios", s.scenarios)
	mux.HandleFunc("/api/scenarios/", s.scenario)
	mux.HandleFunc("/ws/runs/", s.wsRun)
	if s.staticHandler != nil {
		mux.HandleFunc("/", s.static)
	}
	return s.requestID(s.securityHeaders(s.cors(s.auth(mux))))
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"notice": model.SafetyNotice,
	})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	status, err := s.manager.StoreStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "readiness_failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                true,
		"store":             status.Store,
		"migration_version": status.MigrationVersion,
	})
}

func (s *Server) metrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	limit := 100
	snapshot, err := s.collectMetrics(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "metrics_failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"active_runs":                  snapshot.ActiveRuns,
		"listed_runs":                  snapshot.ListedRuns,
		"websocket_connections":        snapshot.WebSocketConnections,
		"snapshot_frames":              snapshot.SnapshotFrames,
		"snapshot_frames_by_run":       snapshot.SnapshotFramesByRun,
		"snapshot_write_count":         snapshot.Runtime.SnapshotWriteCount,
		"snapshot_write_failures":      snapshot.Runtime.SnapshotWriteFailures,
		"snapshot_write_last_ms":       snapshot.Runtime.SnapshotWriteLastMS,
		"snapshot_write_avg_ms":        snapshot.Runtime.SnapshotWriteAvgMS,
		"snapshot_write_max_ms":        snapshot.Runtime.SnapshotWriteMaxMS,
		"http_request_count":           snapshot.RequestCount,
		"http_request_errors":          snapshot.RequestErrors,
		"http_request_duration_avg_ms": snapshot.RequestDurationAvgMS,
		"http_request_duration_max_ms": snapshot.RequestDurationMaxMS,
		"engine_count":                 snapshot.EngineCount,
		"running_engine_count":         snapshot.RunningEngineCount,
		"db_ready":                     snapshot.StoreReady,
		"db_store":                     snapshot.StoreStatus.Store,
		"db_migration_version":         snapshot.StoreStatus.MigrationVersion,
		"db_error":                     snapshot.StoreError,
		"sample_limit":                 snapshot.SampleLimit,
	})
}

func (s *Server) prometheusMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	snapshot, err := s.collectMetrics(r.Context(), 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "metrics_failed", err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	writePrometheusMetrics(w, snapshot)
}

func (s *Server) collectMetrics(ctx context.Context, limit int) (metricsSnapshot, error) {
	runs, err := s.listRuns(ctx, limit)
	if err != nil {
		return metricsSnapshot{}, err
	}
	activeRuns := 0
	snapshotFrames := 0
	snapshotFramesByRun := map[string]int{}
	runtimeMetrics := s.manager.RuntimeMetrics()
	for _, run := range runs {
		if run.Status == model.RunRunning {
			activeRuns++
		}
		if snapshotRange, ok, err := s.manager.SnapshotRange(ctx, run.ID); err == nil && ok {
			snapshotFrames += snapshotRange.Count
			snapshotFramesByRun[run.ID] = snapshotRange.Count
		}
	}
	requestCount := s.requestCount.Load()
	requestDurationAvgMS := 0.0
	if requestCount > 0 {
		requestDurationAvgMS = nsToMS(s.requestDurationTotalNS.Load() / requestCount)
	}
	engineCount, runningEngineCount := s.manager.EngineCounts()
	storeStatus, storeErr := s.manager.StoreStatus(ctx)
	storeReady := storeErr == nil
	storeError := ""
	if storeErr != nil {
		storeError = storeErr.Error()
	}
	return metricsSnapshot{
		ActiveRuns:           activeRuns,
		ListedRuns:           len(runs),
		WebSocketConnections: s.wsConnections.Load(),
		SnapshotFrames:       snapshotFrames,
		SnapshotFramesByRun:  snapshotFramesByRun,
		Runtime:              runtimeMetrics,
		RequestCount:         requestCount,
		RequestErrors:        s.requestErrors.Load(),
		RequestDurationAvgMS: requestDurationAvgMS,
		RequestDurationMaxMS: nsToMS(s.requestDurationMaxNS.Load()),
		EngineCount:          engineCount,
		RunningEngineCount:   runningEngineCount,
		StoreReady:           storeReady,
		StoreStatus:          storeStatus,
		StoreError:           storeError,
		SampleLimit:          limit,
	}, nil
}

func writePrometheusMetrics(w io.Writer, snapshot metricsSnapshot) {
	store := prometheusLabelValue(snapshot.StoreStatus.Store)
	fmt.Fprintln(w, "# HELP ship_sim_http_requests_total Total HTTP requests handled by ShipSystem.")
	fmt.Fprintln(w, "# TYPE ship_sim_http_requests_total counter")
	fmt.Fprintf(w, "ship_sim_http_requests_total %d\n", snapshot.RequestCount)
	fmt.Fprintln(w, "# HELP ship_sim_http_request_errors_total HTTP requests completed with status >= 400.")
	fmt.Fprintln(w, "# TYPE ship_sim_http_request_errors_total counter")
	fmt.Fprintf(w, "ship_sim_http_request_errors_total %d\n", snapshot.RequestErrors)
	fmt.Fprintln(w, "# HELP ship_sim_http_request_duration_seconds HTTP request duration summary.")
	fmt.Fprintln(w, "# TYPE ship_sim_http_request_duration_seconds summary")
	fmt.Fprintf(w, "ship_sim_http_request_duration_seconds_sum %.6f\n", msToSeconds(snapshot.RequestDurationAvgMS*float64(snapshot.RequestCount)))
	fmt.Fprintf(w, "ship_sim_http_request_duration_seconds_count %d\n", snapshot.RequestCount)
	fmt.Fprintln(w, "# HELP ship_sim_http_request_duration_seconds_max Maximum observed HTTP request duration.")
	fmt.Fprintln(w, "# TYPE ship_sim_http_request_duration_seconds_max gauge")
	fmt.Fprintf(w, "ship_sim_http_request_duration_seconds_max %.6f\n", msToSeconds(snapshot.RequestDurationMaxMS))
	fmt.Fprintln(w, "# HELP ship_sim_websocket_connections Active WebSocket connections.")
	fmt.Fprintln(w, "# TYPE ship_sim_websocket_connections gauge")
	fmt.Fprintf(w, "ship_sim_websocket_connections %d\n", snapshot.WebSocketConnections)
	fmt.Fprintln(w, "# HELP ship_sim_runs_active Active running runs in the sampled run list.")
	fmt.Fprintln(w, "# TYPE ship_sim_runs_active gauge")
	fmt.Fprintf(w, "ship_sim_runs_active %d\n", snapshot.ActiveRuns)
	fmt.Fprintln(w, "# HELP ship_sim_runs_listed Runs sampled for metrics.")
	fmt.Fprintln(w, "# TYPE ship_sim_runs_listed gauge")
	fmt.Fprintf(w, "ship_sim_runs_listed %d\n", snapshot.ListedRuns)
	fmt.Fprintln(w, "# HELP ship_sim_engines_total Simulation engines in memory.")
	fmt.Fprintln(w, "# TYPE ship_sim_engines_total gauge")
	fmt.Fprintf(w, "ship_sim_engines_total %d\n", snapshot.EngineCount)
	fmt.Fprintln(w, "# HELP ship_sim_engines_running Running simulation engines in memory.")
	fmt.Fprintln(w, "# TYPE ship_sim_engines_running gauge")
	fmt.Fprintf(w, "ship_sim_engines_running %d\n", snapshot.RunningEngineCount)
	fmt.Fprintln(w, "# HELP ship_sim_snapshot_frames_total Snapshot frames counted in sampled runs.")
	fmt.Fprintln(w, "# TYPE ship_sim_snapshot_frames_total gauge")
	fmt.Fprintf(w, "ship_sim_snapshot_frames_total %d\n", snapshot.SnapshotFrames)
	fmt.Fprintln(w, "# HELP ship_sim_snapshot_writes_total Snapshot write attempts.")
	fmt.Fprintln(w, "# TYPE ship_sim_snapshot_writes_total counter")
	fmt.Fprintf(w, "ship_sim_snapshot_writes_total %d\n", snapshot.Runtime.SnapshotWriteCount)
	fmt.Fprintln(w, "# HELP ship_sim_snapshot_write_failures_total Snapshot write failures.")
	fmt.Fprintln(w, "# TYPE ship_sim_snapshot_write_failures_total counter")
	fmt.Fprintf(w, "ship_sim_snapshot_write_failures_total %d\n", snapshot.Runtime.SnapshotWriteFailures)
	fmt.Fprintln(w, "# HELP ship_sim_snapshot_write_duration_seconds Snapshot write duration gauges.")
	fmt.Fprintln(w, "# TYPE ship_sim_snapshot_write_duration_seconds gauge")
	fmt.Fprintf(w, "ship_sim_snapshot_write_duration_seconds{stat=\"last\"} %.6f\n", msToSeconds(snapshot.Runtime.SnapshotWriteLastMS))
	fmt.Fprintf(w, "ship_sim_snapshot_write_duration_seconds{stat=\"avg\"} %.6f\n", msToSeconds(snapshot.Runtime.SnapshotWriteAvgMS))
	fmt.Fprintf(w, "ship_sim_snapshot_write_duration_seconds{stat=\"max\"} %.6f\n", msToSeconds(snapshot.Runtime.SnapshotWriteMaxMS))
	fmt.Fprintln(w, "# HELP ship_sim_db_ready Database/store readiness state.")
	fmt.Fprintln(w, "# TYPE ship_sim_db_ready gauge")
	fmt.Fprintf(w, "ship_sim_db_ready{store=\"%s\"} %d\n", store, boolGauge(snapshot.StoreReady))
	fmt.Fprintln(w, "# HELP ship_sim_db_migration_version Current store migration version.")
	fmt.Fprintln(w, "# TYPE ship_sim_db_migration_version gauge")
	fmt.Fprintf(w, "ship_sim_db_migration_version{store=\"%s\"} %d\n", store, snapshot.StoreStatus.MigrationVersion)
}

func (s *Server) recordRequest(status int, duration time.Duration) {
	s.requestCount.Add(1)
	if status >= http.StatusBadRequest {
		s.requestErrors.Add(1)
	}
	ns := duration.Nanoseconds()
	s.requestDurationTotalNS.Add(ns)
	for {
		current := s.requestDurationMaxNS.Load()
		if ns <= current || s.requestDurationMaxNS.CompareAndSwap(current, ns) {
			return
		}
	}
}

func nsToMS(ns int64) float64 {
	return float64(ns) / float64(time.Millisecond)
}

func msToSeconds(ms float64) float64 {
	return ms / 1000
}

func boolGauge(value bool) int {
	if value {
		return 1
	}
	return 0
}

func prometheusLabelValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	if value == "" {
		return "unknown"
	}
	return value
}

func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	if s.staticHandler == nil {
		http.NotFound(w, r)
		return
	}
	if r.URL.Path == "/" || !strings.Contains(strings.TrimPrefix(r.URL.Path, "/"), ".") {
		http.ServeFile(w, r, filepath.Join(s.staticDir, "index.html"))
		return
	}
	s.staticHandler.ServeHTTP(w, r)
}

func (s *Server) runs(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/runs" {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodGet:
		runs, err := s.listRuns(r.Context(), intQuery(r, "limit", 50, 100))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list_runs_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, runs)
	case http.MethodPost:
		var req struct {
			Name       string         `json:"name"`
			ScenarioID string         `json:"scenario_id"`
			Scenario   model.Scenario `json:"scenario"`
		}
		if err := s.decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err)
			return
		}
		scenario := req.Scenario
		if req.ScenarioID != "" {
			selected, found, err := s.manager.ScenarioForRun(r.Context(), req.ScenarioID)
			if err != nil {
				writeManagerError(w, err)
				return
			}
			if !found {
				writeError(w, http.StatusNotFound, "scenario_not_found", errors.New("scenario not found"))
				return
			}
			scenario = selected
		}
		run, err := s.manager.CreateRunForOwner(r.Context(), userFromContext(r.Context()), req.Name, scenario)
		if err != nil {
			writeManagerError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, run)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) retentionPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	policy, err := s.retentionPolicyFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_retention_policy", err)
		return
	}
	preview, err := s.manager.PreviewPrune(r.Context(), policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "retention_preview_failed", err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (s *Server) retentionPrune(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	policy, err := s.retentionPolicyFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_retention_policy", err)
		return
	}
	if retentionPolicyEmpty(policy) {
		writeError(w, http.StatusBadRequest, "empty_retention_policy", errors.New("retention policy must include days, cutoff, ended_before, max_track_points_per_run, max_events_per_run, or max_snapshots_per_run"))
		return
	}
	result, err := s.manager.Prune(r.Context(), policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "retention_prune_failed", err)
		return
	}
	s.logger.Info("retention prune completed",
		"owner_id", policy.OwnerID,
		"runs_matched", result.RunsMatched,
		"events_deleted", result.EventsDeleted,
		"track_points_deleted", result.TrackPointsDeleted,
		"contacts_deleted", result.ContactsDeleted,
		"snapshots_deleted", result.SnapshotsDeleted,
	)
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) retentionPolicyFromRequest(r *http.Request) (model.RetentionPolicy, error) {
	var req struct {
		Days                 int    `json:"days"`
		Cutoff               string `json:"cutoff"`
		EndedBefore          string `json:"ended_before"`
		MaxTrackPointsPerRun int    `json:"max_track_points_per_run"`
		MaxEventsPerRun      int    `json:"max_events_per_run"`
		MaxSnapshotsPerRun   int    `json:"max_snapshots_per_run"`
	}
	if r.Method == http.MethodPost {
		if err := s.decodeJSON(r, &req); err != nil {
			return model.RetentionPolicy{}, err
		}
	} else {
		query := r.URL.Query()
		req.Days = intValue(query.Get("days"))
		req.Cutoff = query.Get("cutoff")
		req.EndedBefore = query.Get("ended_before")
		req.MaxTrackPointsPerRun = intValue(query.Get("max_track_points_per_run"))
		req.MaxEventsPerRun = intValue(query.Get("max_events_per_run"))
		req.MaxSnapshotsPerRun = intValue(query.Get("max_snapshots_per_run"))
	}
	if req.Days < 0 {
		return model.RetentionPolicy{}, errors.New("days must be zero or greater")
	}
	if req.MaxTrackPointsPerRun < 0 {
		return model.RetentionPolicy{}, errors.New("max_track_points_per_run must be zero or greater")
	}
	if req.MaxEventsPerRun < 0 {
		return model.RetentionPolicy{}, errors.New("max_events_per_run must be zero or greater")
	}
	if req.MaxSnapshotsPerRun < 0 {
		return model.RetentionPolicy{}, errors.New("max_snapshots_per_run must be zero or greater")
	}
	policy := model.RetentionPolicy{
		MaxTrackPointsPerRun: req.MaxTrackPointsPerRun,
		MaxEventsPerRun:      req.MaxEventsPerRun,
		MaxSnapshotsPerRun:   req.MaxSnapshotsPerRun,
	}
	if s.cfg.AuthEnabled() {
		policy.OwnerID = userFromContext(r.Context())
	}
	if req.Days > 0 {
		policy.Cutoff = time.Now().UTC().Add(-time.Duration(req.Days) * 24 * time.Hour)
	}
	if req.Cutoff != "" {
		cutoff, err := time.Parse(time.RFC3339, req.Cutoff)
		if err != nil {
			return model.RetentionPolicy{}, errors.New("cutoff must be RFC3339")
		}
		policy.Cutoff = cutoff
	}
	if req.EndedBefore != "" {
		endedBefore, err := time.Parse(time.RFC3339, req.EndedBefore)
		if err != nil {
			return model.RetentionPolicy{}, errors.New("ended_before must be RFC3339")
		}
		policy.EndedBefore = endedBefore
	}
	return policy, nil
}

func retentionPolicyEmpty(policy model.RetentionPolicy) bool {
	return policy.Cutoff.IsZero() &&
		policy.EndedBefore.IsZero() &&
		policy.MaxTrackPointsPerRun <= 0 &&
		policy.MaxEventsPerRun <= 0 &&
		policy.MaxSnapshotsPerRun <= 0
}

func (s *Server) scenarios(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/scenarios" {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodGet:
		scenarios, err := s.manager.ListScenarios(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list_scenarios_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, scenarios)
	case http.MethodPost:
		var scenario model.Scenario
		if err := s.decodeJSON(r, &scenario); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err)
			return
		}
		summary, err := s.manager.CreateScenario(r.Context(), userFromContext(r.Context()), scenario)
		if err != nil {
			writeManagerError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, summary)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) scenario(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/scenarios/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			scenario, ok := s.manager.Scenario(r.Context(), id)
			if !ok {
				writeError(w, http.StatusNotFound, "scenario_not_found", errors.New("scenario not found"))
				return
			}
			writeJSON(w, http.StatusOK, scenario)
		case http.MethodPut:
			var scenario model.Scenario
			if err := s.decodeJSON(r, &scenario); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json", err)
				return
			}
			summary, err := s.manager.UpdateScenario(r.Context(), id, userFromContext(r.Context()), scenario)
			if err != nil {
				writeManagerError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, summary)
		default:
			methodNotAllowed(w)
		}
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	switch parts[1] {
	case "copy":
		var req struct {
			Name string `json:"name"`
		}
		if err := s.decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err)
			return
		}
		summary, err := s.manager.CopyScenario(r.Context(), id, req.Name, userFromContext(r.Context()))
		if err != nil {
			writeManagerError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, summary)
	case "enable":
		summary, err := s.manager.SetScenarioEnabled(r.Context(), id, true, userFromContext(r.Context()))
		if err != nil {
			writeManagerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, summary)
	case "disable":
		summary, err := s.manager.SetScenarioEnabled(r.Context(), id, false, userFromContext(r.Context()))
		if err != nil {
			writeManagerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, summary)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) run(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/runs/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	runID := parts[0]
	var authorizedRun model.Run
	if s.cfg.AuthEnabled() {
		var ok bool
		authorizedRun, ok = s.authorizeRun(w, r, runID)
		if !ok {
			return
		}
	}
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		if s.cfg.AuthEnabled() {
			writeJSON(w, http.StatusOK, authorizedRun)
			return
		}
		run, err := s.manager.GetRun(r.Context(), runID)
		if err != nil {
			writeError(w, http.StatusNotFound, "run_not_found", err)
			return
		}
		writeJSON(w, http.StatusOK, run)
		return
	}

	switch parts[1] {
	case "start":
		s.runCommand(w, r, runID, s.manager.Start)
	case "pause":
		s.runCommand(w, r, runID, s.manager.Pause)
	case "stop":
		s.runCommand(w, r, runID, s.manager.Stop)
	case "actions":
		s.actions(w, r, runID)
	case "metadata":
		s.runMetadata(w, r, runID)
	case "annotations":
		s.annotations(w, r, runID)
	case "audit":
		s.auditLogs(w, r, runID)
	case "tracks":
		s.tracks(w, r, runID)
	case "track-points":
		s.trackPoints(w, r, runID)
	case "events":
		s.events(w, r, runID)
	case "snapshots":
		if len(parts) > 2 && parts[2] == "nearest" {
			s.nearestSnapshot(w, r, runID)
			return
		}
		s.snapshots(w, r, runID)
	case "replay":
		s.replay(w, r, runID)
	case "report":
		s.report(w, r, runID)
	case "ws-ticket":
		s.wsTicket(w, r, runID)
	case "zones":
		s.zones(w, r, runID)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) runCommand(w http.ResponseWriter, r *http.Request, runID string, fn func(context.Context, string) (model.Run, error)) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	run, err := fn(r.Context(), runID)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) actions(w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var action model.Action
	if err := s.decodeJSON(r, &action); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err)
		return
	}
	if action.Type == "" {
		writeError(w, http.StatusBadRequest, "action_type_required", errors.New("action type is required"))
		return
	}
	event, err := s.manager.SubmitAction(r.Context(), runID, action)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, event)
}

func (s *Server) runMetadata(w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodPut {
		methodNotAllowed(w)
		return
	}
	var metadata model.RunMetadata
	if err := s.decodeJSON(r, &metadata); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err)
		return
	}
	run, err := s.manager.UpdateRunMetadata(r.Context(), runID, metadata, userFromContext(r.Context()))
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) annotations(w http.ResponseWriter, r *http.Request, runID string) {
	switch r.Method {
	case http.MethodGet:
		annotations, err := s.manager.EventAnnotations(r.Context(), runID)
		if err != nil {
			writeManagerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, annotations)
	case http.MethodPost:
		var annotation model.EventAnnotation
		if err := s.decodeJSON(r, &annotation); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err)
			return
		}
		saved, err := s.manager.AddEventAnnotation(r.Context(), runID, annotation, userFromContext(r.Context()))
		if err != nil {
			writeManagerError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, saved)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) auditLogs(w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	logs, err := s.manager.AuditLogs(r.Context(), model.AuditLogQuery{
		RunID: runID,
		Limit: intQuery(r, "limit", 50, 200),
	})
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

func (s *Server) tracks(w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	tracks, err := s.manager.Tracks(r.Context(), runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_tracks_failed", err)
		return
	}
	writeJSON(w, http.StatusOK, tracks)
}

func (s *Server) events(w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	events, err := s.manager.Events(r.Context(), runID, model.EventQuery{
		Limit:  intQuery(r, "limit", 50, 200),
		Cursor: r.URL.Query().Get("cursor"),
	})
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (s *Server) snapshots(w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	from, err := timeQuery(r, "from")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_time_range", err)
		return
	}
	to, err := timeQuery(r, "to")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_time_range", err)
		return
	}
	frames, err := s.manager.Snapshots(r.Context(), runID, model.SnapshotQuery{
		From:  from,
		To:    to,
		Limit: intQuery(r, "limit", 200, 1000),
	})
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, frames)
}

func (s *Server) nearestSnapshot(w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	at, err := timeQuery(r, "at")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_time_range", err)
		return
	}
	frame, err := s.manager.NearestSnapshot(r.Context(), runID, at)
	if err != nil {
		if strings.Contains(err.Error(), "snapshot not found") {
			writeError(w, http.StatusNotFound, "snapshot_not_found", err)
			return
		}
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, frame)
}

func (s *Server) replay(w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	events, err := s.manager.Events(r.Context(), runID, model.EventQuery{Limit: intQuery(r, "limit", 200, 200)})
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, events.Items)
}

func (s *Server) trackPoints(w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	from, err := timeQuery(r, "from")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_time_range", err)
		return
	}
	to, err := timeQuery(r, "to")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_time_range", err)
		return
	}
	points, err := s.manager.TrackPoints(r.Context(), runID, model.TrackPointQuery{
		TrackID: r.URL.Query().Get("track_id"),
		From:    from,
		To:      to,
		Limit:   intQuery(r, "limit", 200, 1000),
	})
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, points)
}

func (s *Server) report(w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	report, err := s.manager.Report(r.Context(), runID)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "json"
	}
	switch format {
	case "json":
		s.manager.RecordReportExport(r.Context(), runID, userFromContext(r.Context()), format)
		writeJSON(w, http.StatusOK, report)
	case "csv":
		s.manager.RecordReportExport(r.Context(), runID, userFromContext(r.Context()), format)
		writeReportCSV(w, report)
	case "html":
		s.manager.RecordReportExport(r.Context(), runID, userFromContext(r.Context()), format)
		writeReportHTML(w, report)
	case "pdf":
		s.manager.RecordReportExport(r.Context(), runID, userFromContext(r.Context()), format)
		writeReportPDF(w, report)
	default:
		writeError(w, http.StatusBadRequest, "unsupported_report_format", errors.New("format must be json, csv, html, or pdf"))
	}
}

func (s *Server) wsTicket(w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if !s.cfg.AuthEnabled() {
		if _, err := s.manager.GetRun(r.Context(), runID); err != nil {
			writeError(w, http.StatusNotFound, "run_not_found", err)
			return
		}
	}
	ticket, expiresAt, err := s.issueWSTicket(runID, userFromContext(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ws_ticket_failed", err)
		return
	}
	writeJSON(w, http.StatusCreated, wsTicketResponse{Ticket: ticket, ExpiresAt: expiresAt})
}

func (s *Server) zones(w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	zones, err := s.manager.Zones(r.Context(), runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_zones_failed", err)
		return
	}
	writeJSON(w, http.StatusOK, zones)
}

func (s *Server) wsRun(w http.ResponseWriter, r *http.Request) {
	runID := strings.TrimPrefix(r.URL.Path, "/ws/runs/")
	if runID == "" {
		http.NotFound(w, r)
		return
	}
	if !s.checkOrigin(r) {
		writeError(w, http.StatusForbidden, "origin_not_allowed", errors.New("websocket origin is not allowed"))
		return
	}
	if s.cfg.AuthEnabled() {
		userID, ok := s.consumeWSTicket(r.URL.Query().Get("ticket"), runID)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", errors.New("websocket ticket is required"))
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		r = r.WithContext(ctx)
	}
	ch, cancel, err := s.manager.Subscribe(runID)
	if err != nil {
		writeError(w, http.StatusNotFound, "run_not_found", err)
		return
	}
	defer cancel()
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Warn("websocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()
	s.wsConnections.Add(1)
	defer s.wsConnections.Add(-1)

	conn.SetReadLimit(1024)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				_ = conn.Close()
				return
			}
		}
	}()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case snapshot, ok := <-ch:
			if !ok {
				return
			}
			if err := conn.WriteJSON(map[string]any{"type": "snapshot", "payload": snapshot}); err != nil {
				return
			}
		case <-ticker.C:
			if err := conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(5*time.Second)); err != nil {
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) issueWSTicket(runID, userID string) (string, time.Time, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", time.Time{}, err
	}
	ticketValue := base64.RawURLEncoding.EncodeToString(tokenBytes)
	expiresAt := time.Now().UTC().Add(wsTicketTTL)
	s.wsTicketsMu.Lock()
	defer s.wsTicketsMu.Unlock()
	s.pruneExpiredWSTicketsLocked(time.Now().UTC())
	s.wsTickets[ticketValue] = wsTicket{
		RunID:     runID,
		UserID:    userID,
		ExpiresAt: expiresAt,
	}
	return ticketValue, expiresAt, nil
}

func (s *Server) consumeWSTicket(ticketValue, runID string) (string, bool) {
	if strings.TrimSpace(ticketValue) == "" {
		return "", false
	}
	now := time.Now().UTC()
	s.wsTicketsMu.Lock()
	defer s.wsTicketsMu.Unlock()
	s.pruneExpiredWSTicketsLocked(now)
	ticket, ok := s.wsTickets[ticketValue]
	if !ok || ticket.RunID != runID || ticket.ExpiresAt.Before(now) {
		return "", false
	}
	delete(s.wsTickets, ticketValue)
	return ticket.UserID, true
}

func (s *Server) pruneExpiredWSTicketsLocked(now time.Time) {
	for ticketValue, ticket := range s.wsTickets {
		if !ticket.ExpiresAt.After(now) {
			delete(s.wsTickets, ticketValue)
		}
	}
}

func (s *Server) decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	limit := s.cfg.RequestBodyLimit
	if limit <= 0 {
		limit = 1 << 20
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > limit {
		return errors.New("request body too large")
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeReportCSV(w http.ResponseWriter, report model.RunReport) {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="run-%s-report.csv"`, report.Run.ID))
	w.WriteHeader(http.StatusOK)
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"section", "field", "value"})
	_ = writer.Write([]string{"summary", "version", strconv.Itoa(report.Version)})
	_ = writer.Write([]string{"summary", "run_id", report.Run.ID})
	_ = writer.Write([]string{"summary", "name", report.Run.Name})
	_ = writer.Write([]string{"summary", "status", string(report.Run.Status)})
	_ = writer.Write([]string{"summary", "replay_mode", report.ReplayMode})
	_ = writer.Write([]string{"summary", "duration_seconds", strconv.FormatInt(report.DurationSeconds, 10)})
	_ = writer.Write([]string{"summary", "track_count", strconv.Itoa(report.TrackCount)})
	_ = writer.Write([]string{"summary", "high_watermark", strconv.Itoa(report.ThreatSummary.HighWatermark)})
	_ = writer.Write([]string{"assessment", "score", strconv.Itoa(report.Assessment.Score)})
	_ = writer.Write([]string{"assessment", "label", report.Assessment.Label})
	for _, criterion := range report.Assessment.Criteria {
		_ = writer.Write([]string{"assessment_criterion", criterion.Name, strconv.Itoa(criterion.Value)})
	}
	if report.SnapshotCoverage != nil {
		_ = writer.Write([]string{"snapshot", "from", report.SnapshotCoverage.From.Format(time.RFC3339)})
		_ = writer.Write([]string{"snapshot", "to", report.SnapshotCoverage.To.Format(time.RFC3339)})
		_ = writer.Write([]string{"snapshot", "count", strconv.Itoa(report.SnapshotCoverage.Count)})
		_ = writer.Write([]string{"snapshot", "average_interval_ms", fmt.Sprintf("%.2f", report.SnapshotCoverage.AverageIntervalMS)})
	}
	_ = writer.Write([]string{"audit", "event_count", strconv.Itoa(report.EventAudit.EventCount)})
	if report.EventAudit.FirstActionAt != nil {
		_ = writer.Write([]string{"audit", "first_action_at", report.EventAudit.FirstActionAt.Format(time.RFC3339)})
	}
	if report.EventAudit.LastActionAt != nil {
		_ = writer.Write([]string{"audit", "last_action_at", report.EventAudit.LastActionAt.Format(time.RFC3339)})
	}
	for _, stat := range report.ActionStats {
		_ = writer.Write([]string{"action", stat.Type, strconv.Itoa(stat.Count)})
	}
	for _, stat := range report.EventAudit.ActorStats {
		_ = writer.Write([]string{"actor", stat.ActorID, strconv.Itoa(stat.Count)})
	}
	for _, track := range report.FinalTracks {
		_ = writer.Write([]string{"final_track", track.TrackNo, fmt.Sprintf("%s/%s/%d%%", track.Status, track.Threat, int(track.Confidence*100))})
	}
	for _, event := range report.Events {
		payload, _ := json.Marshal(event.Payload)
		_ = writer.Write([]string{"event", event.OccurredAt.Format(time.RFC3339), string(payload)})
	}
	for _, annotation := range report.Annotations {
		_ = writer.Write([]string{"annotation", annotation.CreatedAt.Format(time.RFC3339), annotation.Note})
	}
	for _, auditLog := range report.AuditLogs {
		_ = writer.Write([]string{"audit_log", auditLog.OccurredAt.Format(time.RFC3339), auditLog.Action})
	}
	writer.Flush()
}

func writeReportHTML(w http.ResponseWriter, report model.RunReport) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="run-%s-report.html"`, report.Run.ID))
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "<!doctype html><html><head><meta charset=\"utf-8\"><title>Training Report %s</title>", html.EscapeString(report.Run.ID))
	fmt.Fprint(w, "<style>body{font-family:Arial,sans-serif;color:#102033;margin:32px;line-height:1.4}table{border-collapse:collapse;width:100%;margin:16px 0}td,th{border:1px solid #ccd7e3;padding:8px;text-align:left}h1,h2{margin-bottom:8px}.notice{border:1px solid #8fb3d9;background:#eef6ff;padding:10px}</style></head><body>")
	fmt.Fprintf(w, "<h1>%s</h1><p class=\"notice\">%s</p>", html.EscapeString(report.Run.Name), html.EscapeString(report.SafetyNotice))
	fmt.Fprintf(w, "<table><tr><th>Run ID</th><td>%s</td></tr><tr><th>Status</th><td>%s</td></tr><tr><th>Report Version</th><td>%d</td></tr><tr><th>Duration</th><td>%ds</td></tr><tr><th>Assessment</th><td>%s (%d)</td></tr></table>",
		html.EscapeString(report.Run.ID), html.EscapeString(string(report.Run.Status)), report.Version, report.DurationSeconds, html.EscapeString(report.Assessment.Label), report.Assessment.Score)
	fmt.Fprint(w, "<h2>Run Metadata</h2><table><tr><th>Tags</th><td>")
	fmt.Fprint(w, html.EscapeString(strings.Join(report.Run.Tags, ", ")))
	fmt.Fprint(w, "</td></tr><tr><th>Trainees</th><td>")
	fmt.Fprint(w, html.EscapeString(strings.Join(report.Run.Trainees, ", ")))
	fmt.Fprint(w, "</td></tr><tr><th>Instructor Notes</th><td>")
	fmt.Fprint(w, html.EscapeString(report.Run.InstructorNotes))
	fmt.Fprint(w, "</td></tr></table>")
	fmt.Fprint(w, "<h2>Assessment Criteria</h2><table><tr><th>Name</th><th>Value</th><th>Note</th></tr>")
	for _, criterion := range report.Assessment.Criteria {
		fmt.Fprintf(w, "<tr><td>%s</td><td>%d</td><td>%s</td></tr>", html.EscapeString(criterion.Name), criterion.Value, html.EscapeString(criterion.Note))
	}
	fmt.Fprint(w, "</table><h2>Annotations</h2><table><tr><th>Time</th><th>Actor</th><th>Note</th></tr>")
	for _, annotation := range report.Annotations {
		fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td><td>%s</td></tr>", annotation.CreatedAt.Format(time.RFC3339), html.EscapeString(annotation.ActorID), html.EscapeString(annotation.Note))
	}
	fmt.Fprint(w, "</table><h2>Audit Log</h2><table><tr><th>Time</th><th>Actor</th><th>Action</th><th>Target</th></tr>")
	for _, auditLog := range report.AuditLogs {
		fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>", auditLog.OccurredAt.Format(time.RFC3339), html.EscapeString(auditLog.ActorID), html.EscapeString(auditLog.Action), html.EscapeString(auditLog.TargetID))
	}
	fmt.Fprint(w, "</table></body></html>")
}

func writeReportPDF(w http.ResponseWriter, report model.RunReport) {
	lines := []string{
		"ShipSystem Training Report",
		"Run: " + report.Run.Name,
		"Run ID: " + report.Run.ID,
		"Status: " + string(report.Run.Status),
		"Report Version: " + strconv.Itoa(report.Version),
		"Assessment: " + report.Assessment.Label + " " + strconv.Itoa(report.Assessment.Score),
		"Duration Seconds: " + strconv.FormatInt(report.DurationSeconds, 10),
		"Tracks: " + strconv.Itoa(report.TrackCount),
		"Annotations: " + strconv.Itoa(len(report.Annotations)),
		"Audit Logs: " + strconv.Itoa(len(report.AuditLogs)),
		"Safety: " + report.SafetyNotice,
	}
	pdf := simplePDF(lines)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="run-%s-report.pdf"`, report.Run.ID))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdf)
}

func simplePDF(lines []string) []byte {
	var content strings.Builder
	content.WriteString("BT /F1 12 Tf 72 760 Td 16 TL\n")
	for i, line := range lines {
		if i > 0 {
			content.WriteString("T*\n")
		}
		content.WriteString("(")
		content.WriteString(pdfEscape(line))
		content.WriteString(") Tj\n")
	}
	content.WriteString("ET\n")
	stream := content.String()
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream),
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for i, object := range objects {
		offsets[i+1] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for i := 1; i <= len(objects); i++ {
		fmt.Fprintf(&out, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&out, "trailer << /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return out.Bytes()
}

func pdfEscape(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "(", `\(`)
	value = strings.ReplaceAll(value, ")", `\)`)
	if len(value) > 100 {
		value = value[:100]
	}
	return value
}

type errorPayload struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Details []string `json:"details,omitempty"`
}

func writeManagerError(w http.ResponseWriter, err error) {
	var validation sim.ValidationError
	if errors.As(err, &validation) {
		writeErrorDetails(w, http.StatusBadRequest, "validation_failed", "validation failed", validation.Details)
		return
	}
	if strings.Contains(err.Error(), "scenario not found") {
		writeError(w, http.StatusNotFound, "scenario_not_found", err)
		return
	}
	if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "engine not found") {
		writeError(w, http.StatusNotFound, "run_not_found", err)
		return
	}
	writeError(w, http.StatusInternalServerError, "request_failed", err)
}

func writeError(w http.ResponseWriter, status int, code string, err error) {
	writeErrorDetails(w, status, code, err.Error(), nil)
}

func writeErrorDetails(w http.ResponseWriter, status int, code, message string, details []string) {
	writeJSON(w, status, errorPayload{Error: errorBody{
		Code:    code,
		Message: message,
		Details: details,
	}})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", errors.New("method not allowed"))
}

type contextKey string

const (
	requestIDKey        contextKey = "request_id"
	userIDKey           contextKey = "user_id"
	requestLogFieldsKey contextKey = "request_log_fields"
)

type requestLogFields struct {
	UserID string
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (s *Server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if requestID == "" {
			requestID = fmt.Sprintf("%d", time.Now().UnixNano())
		}
		fields := &requestLogFields{}
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		ctx = context.WithValue(ctx, requestLogFieldsKey, fields)
		w.Header().Set("X-Request-ID", requestID)
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(recorder, r.WithContext(ctx))
		duration := time.Since(start)
		s.recordRequest(recorder.status, duration)
		s.logger.Info("request completed",
			"request_id", requestID,
			"user_id", fields.UserID,
			"run_id", runIDFromPath(r.URL.Path),
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration_ms", duration.Milliseconds(),
		)
	})
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.requiresAuth(r) {
			next.ServeHTTP(w, r)
			return
		}
		userID, ok := s.authenticate(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", errors.New("authentication is required"))
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		if fields, ok := r.Context().Value(requestLogFieldsKey).(*requestLogFields); ok {
			fields.UserID = userID
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) requiresAuth(r *http.Request) bool {
	if !s.cfg.AuthEnabled() {
		return false
	}
	path := r.URL.Path
	return path == "/readyz" || path == "/metrics" || path == "/metrics/prometheus" || path == "/api/runs" || path == "/api/scenarios" ||
		strings.HasPrefix(path, "/api/retention/") ||
		strings.HasPrefix(path, "/api/runs/") || strings.HasPrefix(path, "/api/scenarios/")
}

func (s *Server) authenticate(r *http.Request) (string, bool) {
	switch s.cfg.AuthMode {
	case config.AuthToken:
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" {
			token = r.Header.Get("X-Ship-Sim-Token")
		}
		if token != "" && token == s.cfg.AuthToken {
			return "token-user", true
		}
	case config.AuthProxy:
		user := strings.TrimSpace(r.Header.Get(s.cfg.AuthUserHeader))
		if user != "" {
			return user, true
		}
	case config.AuthOff:
		return "", true
	}
	return "", false
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self' ws: wss: https:; img-src 'self' data: blob: https:; style-src 'self' 'unsafe-inline'; script-src 'self'; worker-src 'self' blob:; font-src 'self' data:; object-src 'none'; base-uri 'self'; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func userFromContext(ctx context.Context) string {
	if userID, ok := ctx.Value(userIDKey).(string); ok {
		return userID
	}
	return ""
}

func runIDFromPath(path string) string {
	for _, prefix := range []string{"/api/runs/", "/ws/runs/"} {
		if strings.HasPrefix(path, prefix) {
			rest := strings.TrimPrefix(path, prefix)
			if rest == "" {
				return ""
			}
			if idx := strings.IndexByte(rest, '/'); idx >= 0 {
				return rest[:idx]
			}
			return rest
		}
	}
	return ""
}

func (s *Server) listRuns(ctx context.Context, limit int) ([]model.Run, error) {
	if s.cfg.AuthEnabled() {
		return s.manager.ListRunsForOwner(ctx, limit, userFromContext(ctx))
	}
	return s.manager.ListRuns(ctx, limit)
}

func (s *Server) filterRunsForUser(ctx context.Context, runs []model.Run) []model.Run {
	if !s.cfg.AuthEnabled() {
		return runs
	}
	userID := userFromContext(ctx)
	filtered := make([]model.Run, 0, len(runs))
	for _, run := range runs {
		if run.OwnerID == userID {
			filtered = append(filtered, run)
		}
	}
	return filtered
}

func (s *Server) authorizeRun(w http.ResponseWriter, r *http.Request, runID string) (model.Run, bool) {
	run, err := s.manager.GetRunForOwner(r.Context(), runID, userFromContext(r.Context()))
	if err != nil {
		writeError(w, http.StatusNotFound, "run_not_found", err)
		return model.Run{}, false
	}
	return run, true
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if s.allowOrigin(origin, r.Host) {
			if origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			}
		} else if r.Method == http.MethodOptions {
			writeError(w, http.StatusForbidden, "origin_not_allowed", errors.New("origin is not allowed"))
			return
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Ship-Sim-Token")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) checkOrigin(r *http.Request) bool {
	return s.allowOrigin(r.Header.Get("Origin"), r.Host)
}

func (s *Server) allowOrigin(origin, host string) bool {
	if origin == "" {
		return true
	}
	if _, ok := s.allowedOrigins["*"]; ok {
		return true
	}
	if _, ok := s.allowedOrigins[origin]; ok {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && parsed.Host == host
}

func parseAllowedOrigins(configured []string) map[string]struct{} {
	values := []string{
		"http://localhost:5173",
		"http://127.0.0.1:5173",
		"http://localhost:4173",
		"http://127.0.0.1:4173",
	}
	if len(configured) > 0 {
		values = configured
	}
	out := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out[value] = struct{}{}
		}
	}
	return out
}

func intQuery(r *http.Request, key string, fallback, maxValue int) int {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	if parsed > maxValue {
		return maxValue
	}
	return parsed
}

func intValue(value string) int {
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

func timeQuery(r *http.Request, key string) (time.Time, error) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, errors.New(key + " must be RFC3339")
	}
	return parsed, nil
}
