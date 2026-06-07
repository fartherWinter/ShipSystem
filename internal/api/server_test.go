package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"shipsim/internal/config"
	"shipsim/internal/model"
	"shipsim/internal/sim"
	"shipsim/internal/store"
)

func TestRunAPI(t *testing.T) {
	server := NewServer(sim.NewManager(store.NewMemory(), slog.Default()), slog.Default())
	ts := httptest.NewServer(server.Routes())
	defer ts.Close()

	res, err := http.Post(ts.URL+"/api/runs", "application/json", bytes.NewBufferString(`{}`))
	if err != nil {
		t.Fatalf("create run request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", res.StatusCode)
	}
	var run struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&run); err != nil {
		t.Fatalf("decode run: %v", err)
	}
	if run.ID == "" {
		t.Fatal("expected run id")
	}

	res, err = http.Post(ts.URL+"/api/runs/"+run.ID+"/start", "application/json", nil)
	if err != nil {
		t.Fatalf("start run request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	res, err = http.Get(ts.URL + "/api/runs?limit=10")
	if err != nil {
		t.Fatalf("list runs request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected list runs 200, got %d", res.StatusCode)
	}
}

func TestStructuredValidationError(t *testing.T) {
	server := NewServer(sim.NewManager(store.NewMemory(), slog.Default()), slog.Default())
	ts := httptest.NewServer(server.Routes())
	defer ts.Close()

	body := `{"scenario":{"name":"bad","tick_hz":100,"snapshot_hz":10}}`
	res, err := http.Post(ts.URL+"/api/runs", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("create invalid run request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.StatusCode)
	}
	var payload struct {
		Error struct {
			Code    string   `json:"code"`
			Details []string `json:"details"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if payload.Error.Code != "validation_failed" {
		t.Fatalf("expected validation_failed, got %q", payload.Error.Code)
	}
	if len(payload.Error.Details) == 0 {
		t.Fatal("expected validation details")
	}
}

func TestRejectsUnknownJSONFields(t *testing.T) {
	server := NewServer(sim.NewManager(store.NewMemory(), slog.Default()), slog.Default())
	ts := httptest.NewServer(server.Routes())
	defer ts.Close()

	res, err := http.Post(ts.URL+"/api/runs", "application/json", bytes.NewBufferString(`{"unknown":true}`))
	if err != nil {
		t.Fatalf("create invalid run request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.StatusCode)
	}
}

func TestReadinessAndScenariosAPI(t *testing.T) {
	server := NewServer(sim.NewManager(store.NewMemory(), slog.Default()), slog.Default())
	ts := httptest.NewServer(server.Routes())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/readyz")
	if err != nil {
		t.Fatalf("ready request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected ready 200, got %d", res.StatusCode)
	}
	var ready struct {
		OK               bool   `json:"ok"`
		Store            string `json:"store"`
		MigrationVersion int    `json:"migration_version"`
	}
	if err := json.NewDecoder(res.Body).Decode(&ready); err != nil {
		t.Fatalf("decode ready: %v", err)
	}
	if !ready.OK || ready.Store != "memory" || ready.MigrationVersion != 0 {
		t.Fatalf("unexpected ready payload: %+v", ready)
	}

	res, err = http.Get(ts.URL + "/api/scenarios")
	if err != nil {
		t.Fatalf("scenarios request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected scenarios 200, got %d", res.StatusCode)
	}
	var scenarios []model.ScenarioSummary
	if err := json.NewDecoder(res.Body).Decode(&scenarios); err != nil {
		t.Fatalf("decode scenarios: %v", err)
	}
	if len(scenarios) == 0 {
		t.Fatal("expected default scenario")
	}

	res, err = http.Get(ts.URL + "/api/scenarios/default")
	if err != nil {
		t.Fatalf("scenario detail request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected scenario detail 200, got %d", res.StatusCode)
	}
	var scenario model.Scenario
	if err := json.NewDecoder(res.Body).Decode(&scenario); err != nil {
		t.Fatalf("decode scenario detail: %v", err)
	}
	if scenario.ID != "default" {
		t.Fatalf("expected default scenario id, got %q", scenario.ID)
	}

	res, err = http.Post(ts.URL+"/api/runs", "application/json", bytes.NewBufferString(`{"scenario_id":"default"}`))
	if err != nil {
		t.Fatalf("create run from scenario request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected scenario run 201, got %d", res.StatusCode)
	}
}

func TestTokenAuthProtectsAPIAndWebSocket(t *testing.T) {
	cfg := config.Default()
	cfg.AuthMode = config.AuthToken
	cfg.AuthToken = "secret"
	server := NewServerWithConfig(sim.NewManager(store.NewMemory(), slog.Default()), slog.Default(), cfg)
	ts := httptest.NewServer(server.Routes())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/runs")
	if err != nil {
		t.Fatalf("unauthenticated runs request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated 401, got %d", res.StatusCode)
	}

	res, err = http.Get(ts.URL + "/api/runs?access_token=secret")
	if err != nil {
		t.Fatalf("query token runs request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected query token 401, got %d", res.StatusCode)
	}

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/runs", nil)
	if err != nil {
		t.Fatalf("new wrong-token request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer wrong")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("wrong-token runs request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected wrong token 401, got %d", res.StatusCode)
	}

	req, err = http.NewRequest(http.MethodGet, ts.URL+"/api/runs", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("authenticated runs request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected authenticated 200, got %d", res.StatusCode)
	}
}

func TestSecurityHeaders(t *testing.T) {
	server := NewServer(sim.NewManager(store.NewMemory(), slog.Default()), slog.Default())
	ts := httptest.NewServer(server.Routes())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("health request: %v", err)
	}
	defer res.Body.Close()
	if res.Header.Get("Content-Security-Policy") == "" {
		t.Fatal("expected Content-Security-Policy")
	}
	if got := res.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("expected nosniff, got %q", got)
	}
	if got := res.Header.Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("expected no-referrer, got %q", got)
	}
	if got := res.Header.Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("expected DENY, got %q", got)
	}
}

func TestStaticIndexFallbackDoesNotRedirectLoop(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.WriteFile(staticDir+"/index.html", []byte("<html><body>ShipSim</body></html>"), 0o600); err != nil {
		t.Fatalf("write index: %v", err)
	}
	cfg := config.Default()
	cfg.StaticDir = staticDir
	server := NewServerWithConfig(sim.NewManager(store.NewMemory(), slog.Default()), slog.Default(), cfg)
	ts := httptest.NewServer(server.Routes())
	defer ts.Close()

	for _, path := range []string{"/", "/runs/demo"} {
		res, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("static request %s: %v", path, err)
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("expected static %s 200, got %d", path, res.StatusCode)
		}
	}
}

func TestEventsAndTrackPointsAPI(t *testing.T) {
	server := NewServer(sim.NewManager(store.NewMemory(), slog.Default()), slog.Default())
	ts := httptest.NewServer(server.Routes())
	defer ts.Close()

	run := createTestRun(t, ts.URL)
	if _, err := http.Post(ts.URL+"/api/runs/"+run.ID+"/start", "application/json", nil); err != nil {
		t.Fatalf("start run request: %v", err)
	}
	time.Sleep(250 * time.Millisecond)
	if _, err := http.Post(ts.URL+"/api/runs/"+run.ID+"/actions", "application/json", bytes.NewBufferString(`{"type":"training_response"}`)); err != nil {
		t.Fatalf("action request: %v", err)
	}

	res, err := http.Get(ts.URL + "/api/runs/" + run.ID + "/events?limit=1")
	if err != nil {
		t.Fatalf("events request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected events 200, got %d", res.StatusCode)
	}
	var page model.EventPage
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		t.Fatalf("decode events: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected one event, got %d", len(page.Items))
	}

	res, err = http.Get(ts.URL + "/api/runs/" + run.ID + "/track-points?limit=10")
	if err != nil {
		t.Fatalf("track points request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected track points 200, got %d", res.StatusCode)
	}
	var points []model.TrackPoint
	if err := json.NewDecoder(res.Body).Decode(&points); err != nil {
		t.Fatalf("decode track points: %v", err)
	}
	if len(points) == 0 {
		t.Fatal("expected saved track points")
	}
}

func TestSnapshotsAndReportAPI(t *testing.T) {
	server := NewServer(sim.NewManager(store.NewMemory(), slog.Default()), slog.Default())
	ts := httptest.NewServer(server.Routes())
	defer ts.Close()

	run := createTestRun(t, ts.URL)
	if _, err := http.Post(ts.URL+"/api/runs/"+run.ID+"/start", "application/json", nil); err != nil {
		t.Fatalf("start run request: %v", err)
	}
	time.Sleep(250 * time.Millisecond)
	if _, err := http.Post(ts.URL+"/api/runs/"+run.ID+"/actions", "application/json", bytes.NewBufferString(`{"type":"decoy","actor_id":"tester"}`)); err != nil {
		t.Fatalf("action request: %v", err)
	}

	res, err := http.Get(ts.URL + "/api/runs/" + run.ID + "/snapshots?limit=5")
	if err != nil {
		t.Fatalf("snapshots request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected snapshots 200, got %d", res.StatusCode)
	}
	var frames []model.SnapshotFrame
	if err := json.NewDecoder(res.Body).Decode(&frames); err != nil {
		t.Fatalf("decode snapshots: %v", err)
	}
	if len(frames) == 0 || len(frames[0].Tracks) == 0 {
		t.Fatalf("expected snapshot frames with tracks, got %+v", frames)
	}
	if len(frames) > 1 && frames[0].SampledAt.After(frames[1].SampledAt) {
		t.Fatalf("expected snapshots in ascending order, got %+v", frames[:2])
	}

	windowFrom := urlQueryEscape(frames[0].SampledAt.Format(time.RFC3339Nano))
	windowTo := urlQueryEscape(frames[len(frames)-1].SampledAt.Format(time.RFC3339Nano))
	res, err = http.Get(ts.URL + "/api/runs/" + run.ID + "/snapshots?from=" + windowFrom + "&to=" + windowTo + "&limit=20")
	if err != nil {
		t.Fatalf("windowed snapshots request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected windowed snapshots 200, got %d", res.StatusCode)
	}
	var windowFrames []model.SnapshotFrame
	if err := json.NewDecoder(res.Body).Decode(&windowFrames); err != nil {
		t.Fatalf("decode windowed snapshots: %v", err)
	}
	if len(windowFrames) == 0 {
		t.Fatal("expected windowed snapshots")
	}

	res, err = http.Get(ts.URL + "/api/runs/" + run.ID + "/snapshots/nearest?at=" + urlQueryEscape(frames[0].SampledAt.Format(time.RFC3339Nano)))
	if err != nil {
		t.Fatalf("nearest snapshot request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected nearest snapshot 200, got %d", res.StatusCode)
	}
	var nearest model.SnapshotFrame
	if err := json.NewDecoder(res.Body).Decode(&nearest); err != nil {
		t.Fatalf("decode nearest snapshot: %v", err)
	}
	if nearest.RunID != run.ID {
		t.Fatalf("expected nearest snapshot for run %s, got %s", run.ID, nearest.RunID)
	}

	res, err = http.Get(ts.URL + "/api/runs/" + run.ID + "/report")
	if err != nil {
		t.Fatalf("report request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected report 200, got %d", res.StatusCode)
	}
	var report model.RunReport
	if err := json.NewDecoder(res.Body).Decode(&report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if report.Version != 1 || report.ReplayMode != "snapshot" || report.SnapshotRange == nil || report.SnapshotCoverage == nil || report.TrackCount == 0 {
		t.Fatalf("unexpected snapshot report: %+v", report)
	}
	if report.SnapshotCoverage.Count != report.SnapshotRange.Count || report.SnapshotCoverage.From != report.SnapshotRange.From || report.SnapshotCoverage.To != report.SnapshotRange.To {
		t.Fatalf("expected snapshot coverage to mirror range, got coverage=%+v range=%+v", report.SnapshotCoverage, report.SnapshotRange)
	}
	if len(report.ActionStats) == 0 {
		t.Fatal("expected action stats")
	}
	if report.EventAudit.EventCount == 0 || len(report.EventAudit.ActionStats) == 0 || report.EventAudit.FirstActionAt == nil || report.EventAudit.LastActionAt == nil {
		t.Fatalf("expected event audit summary, got %+v", report.EventAudit)
	}
	if !hasActorStat(report.EventAudit.ActorStats, "tester") {
		t.Fatalf("expected actor stats to include tester, got %+v", report.EventAudit.ActorStats)
	}

	res, err = http.Get(ts.URL + "/api/runs/" + run.ID + "/report?format=csv")
	if err != nil {
		t.Fatalf("csv report request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected csv report 200, got %d", res.StatusCode)
	}
	if contentType := res.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "text/csv") {
		t.Fatalf("expected text/csv response, got %q", contentType)
	}
	csvBody, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read csv report: %v", err)
	}
	if !strings.Contains(string(csvBody), "summary,run_id,"+run.ID) || !strings.Contains(string(csvBody), "summary,version,1") || !strings.Contains(string(csvBody), "actor,tester,1") {
		t.Fatalf("expected csv report to include run id, got %q", string(csvBody))
	}

	res, err = http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("metrics request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected metrics 200, got %d", res.StatusCode)
	}
	var metrics map[string]any
	if err := json.NewDecoder(res.Body).Decode(&metrics); err != nil {
		t.Fatalf("decode metrics: %v", err)
	}
	for _, key := range []string{
		"active_runs",
		"listed_runs",
		"websocket_connections",
		"snapshot_frames",
		"snapshot_frames_by_run",
		"snapshot_write_count",
		"snapshot_write_failures",
		"snapshot_write_last_ms",
		"snapshot_write_avg_ms",
		"snapshot_write_max_ms",
		"http_request_count",
		"http_request_errors",
		"http_request_duration_avg_ms",
		"http_request_duration_max_ms",
		"engine_count",
		"running_engine_count",
		"db_ready",
		"db_store",
		"db_migration_version",
		"sample_limit",
	} {
		if _, ok := metrics[key]; !ok {
			t.Fatalf("expected metrics key %q in %+v", key, metrics)
		}
	}
	if metrics["snapshot_frames"].(float64) == 0 || metrics["snapshot_write_count"].(float64) == 0 {
		t.Fatalf("expected snapshot metrics to be nonzero, got %+v", metrics)
	}
	if framesByRun, ok := metrics["snapshot_frames_by_run"].(map[string]any); !ok || framesByRun[run.ID].(float64) == 0 {
		t.Fatalf("expected per-run snapshot metrics for %s, got %+v", run.ID, metrics["snapshot_frames_by_run"])
	}

	res, err = http.Get(ts.URL + "/metrics/prometheus")
	if err != nil {
		t.Fatalf("prometheus metrics request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected prometheus metrics 200, got %d", res.StatusCode)
	}
	if contentType := res.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "text/plain") {
		t.Fatalf("expected prometheus text response, got %q", contentType)
	}
	prometheusBody, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read prometheus metrics: %v", err)
	}
	for _, metric := range []string{
		"ship_sim_http_requests_total",
		"ship_sim_http_request_errors_total",
		"ship_sim_websocket_connections",
		"ship_sim_snapshot_writes_total",
		"ship_sim_snapshot_write_failures_total",
		"ship_sim_engines_total",
		"ship_sim_db_ready",
	} {
		if !strings.Contains(string(prometheusBody), metric) {
			t.Fatalf("expected prometheus metric %q in %s", metric, string(prometheusBody))
		}
	}
}

func TestLegacyReportWithoutSnapshots(t *testing.T) {
	server := NewServer(sim.NewManager(store.NewMemory(), slog.Default()), slog.Default())
	ts := httptest.NewServer(server.Routes())
	defer ts.Close()

	run := createTestRun(t, ts.URL)
	res, err := http.Get(ts.URL + "/api/runs/" + run.ID + "/report")
	if err != nil {
		t.Fatalf("legacy report request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected legacy report 200, got %d", res.StatusCode)
	}
	var report model.RunReport
	if err := json.NewDecoder(res.Body).Decode(&report); err != nil {
		t.Fatalf("decode legacy report: %v", err)
	}
	if report.Version != 1 || report.ReplayMode != "legacy" || report.SnapshotRange != nil || report.SnapshotCoverage != nil {
		t.Fatalf("unexpected legacy report: %+v", report)
	}
}

func TestRetentionPreviewAndPruneAPI(t *testing.T) {
	server := NewServer(sim.NewManager(store.NewMemory(), slog.Default()), slog.Default())
	ts := httptest.NewServer(server.Routes())
	defer ts.Close()

	run := createTestRun(t, ts.URL)
	if _, err := http.Post(ts.URL+"/api/runs/"+run.ID+"/start", "application/json", nil); err != nil {
		t.Fatalf("start run request: %v", err)
	}
	time.Sleep(250 * time.Millisecond)
	if _, err := http.Post(ts.URL+"/api/runs/"+run.ID+"/actions", "application/json", bytes.NewBufferString(`{"type":"maneuver"}`)); err != nil {
		t.Fatalf("action request: %v", err)
	}
	if _, err := http.Post(ts.URL+"/api/runs/"+run.ID+"/stop", "application/json", nil); err != nil {
		t.Fatalf("stop run request: %v", err)
	}
	capacityRes, err := http.Get(ts.URL + "/api/retention/preview?max_track_points_per_run=1&max_events_per_run=1&max_snapshots_per_run=1")
	if err != nil {
		t.Fatalf("retention capacity preview request: %v", err)
	}
	defer capacityRes.Body.Close()
	if capacityRes.StatusCode != http.StatusOK {
		t.Fatalf("expected retention capacity preview 200, got %d", capacityRes.StatusCode)
	}
	var capacityPreview model.RetentionPreview
	if err := json.NewDecoder(capacityRes.Body).Decode(&capacityPreview); err != nil {
		t.Fatalf("decode retention capacity preview: %v", err)
	}
	if capacityPreview.TrackPointsMatched == 0 || capacityPreview.SnapshotsMatched == 0 {
		t.Fatalf("expected retention capacity preview to match excess history, got %+v", capacityPreview)
	}
	endedBefore := time.Now().UTC().Add(time.Second).Format(time.RFC3339)

	res, err := http.Get(ts.URL + "/api/retention/preview?ended_before=" + urlQueryEscape(endedBefore))
	if err != nil {
		t.Fatalf("retention preview request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected retention preview 200, got %d", res.StatusCode)
	}
	var preview model.RetentionPreview
	if err := json.NewDecoder(res.Body).Decode(&preview); err != nil {
		t.Fatalf("decode retention preview: %v", err)
	}
	if preview.RunsMatched != 1 || preview.EventsMatched == 0 || preview.TrackPointsMatched == 0 || preview.ContactsMatched == 0 || preview.SnapshotsMatched == 0 {
		t.Fatalf("expected retention preview to match saved history, got %+v", preview)
	}

	body := bytes.NewBufferString(`{"ended_before":"` + endedBefore + `"}`)
	res, err = http.Post(ts.URL+"/api/retention/prune", "application/json", body)
	if err != nil {
		t.Fatalf("retention prune request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected retention prune 200, got %d", res.StatusCode)
	}
	var result model.RetentionResult
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		t.Fatalf("decode retention result: %v", err)
	}
	if result.RunsMatched != 1 || result.EventsDeleted == 0 || result.TrackPointsDeleted == 0 || result.ContactsDeleted == 0 || result.SnapshotsDeleted == 0 {
		t.Fatalf("expected retention prune to delete saved history, got %+v", result)
	}
}

func TestAuthOwnerIsolation(t *testing.T) {
	manager := sim.NewManager(store.NewMemory(), slog.Default())
	cfg := config.Default()
	cfg.AuthMode = config.AuthToken
	cfg.AuthToken = "secret"
	server := NewServerWithConfig(manager, slog.Default(), cfg)
	ts := httptest.NewServer(server.Routes())
	defer ts.Close()

	req := authenticatedRequest(t, http.MethodPost, ts.URL+"/api/runs", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create authenticated run: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected authenticated create 201, got %d", res.StatusCode)
	}
	var ownedRun model.Run
	if err := json.NewDecoder(res.Body).Decode(&ownedRun); err != nil {
		t.Fatalf("decode owned run: %v", err)
	}
	if ownedRun.OwnerID != "token-user" {
		t.Fatalf("expected token-user owner, got %q", ownedRun.OwnerID)
	}

	res, err = http.Get(ts.URL + "/api/runs/" + ownedRun.ID + "/report?format=csv")
	if err != nil {
		t.Fatalf("unauthenticated report export: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated report 401, got %d", res.StatusCode)
	}

	req, err = http.NewRequest(http.MethodGet, ts.URL+"/api/runs/"+ownedRun.ID+"/report?format=csv", nil)
	if err != nil {
		t.Fatalf("new wrong-token report request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer wrong")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("wrong-token report export: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected wrong-token report 401, got %d", res.StatusCode)
	}

	req = authenticatedRequest(t, http.MethodGet, ts.URL+"/api/runs/"+ownedRun.ID+"/report?format=csv", nil)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("authenticated report export: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected authenticated report 200, got %d", res.StatusCode)
	}
	if contentType := res.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "text/csv") {
		t.Fatalf("expected csv report, got %q", contentType)
	}

	otherRun, err := manager.CreateRunForOwner(context.Background(), "other-user", "other", sim.DefaultScenario())
	if err != nil {
		t.Fatalf("create other run: %v", err)
	}

	req = authenticatedRequest(t, http.MethodGet, ts.URL+"/api/runs?limit=10", nil)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("list authenticated runs: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected authenticated list 200, got %d", res.StatusCode)
	}
	var runs []model.Run
	if err := json.NewDecoder(res.Body).Decode(&runs); err != nil {
		t.Fatalf("decode authenticated runs: %v", err)
	}
	if len(runs) != 1 || runs[0].ID != ownedRun.ID {
		t.Fatalf("expected only owned run, got %+v", runs)
	}

	for _, suffix := range []string{
		"",
		"/events",
		"/snapshots",
		"/snapshots/nearest",
		"/report",
		"/zones",
		"/tracks",
		"/track-points",
	} {
		req = authenticatedRequest(t, http.MethodGet, ts.URL+"/api/runs/"+otherRun.ID+suffix, nil)
		res, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("get other run %s: %v", suffix, err)
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusNotFound {
			t.Fatalf("expected other run %s 404, got %d", suffix, res.StatusCode)
		}
	}

	req = authenticatedRequest(t, http.MethodGet, ts.URL+"/metrics", nil)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("authenticated metrics: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected authenticated metrics 200, got %d", res.StatusCode)
	}
	var metrics map[string]any
	if err := json.NewDecoder(res.Body).Decode(&metrics); err != nil {
		t.Fatalf("decode authenticated metrics: %v", err)
	}
	if metrics["listed_runs"].(float64) != 1 {
		t.Fatalf("expected metrics to list only owned runs, got %+v", metrics)
	}

	req = authenticatedRequest(t, http.MethodGet, ts.URL+"/metrics/prometheus", nil)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("authenticated prometheus metrics: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected authenticated prometheus metrics 200, got %d", res.StatusCode)
	}
}

func TestRequestLogsIncludeAuditFieldsWithoutSensitiveValues(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	cfg := config.Default()
	cfg.AuthMode = config.AuthToken
	cfg.AuthToken = "secret"
	server := NewServerWithConfig(sim.NewManager(store.NewMemory(), logger), logger, cfg)
	ts := httptest.NewServer(server.Routes())
	defer ts.Close()

	req := authenticatedRequest(t, http.MethodPost, ts.URL+"/api/runs", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create authenticated run: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected create 201, got %d", res.StatusCode)
	}
	var run model.Run
	if err := json.NewDecoder(res.Body).Decode(&run); err != nil {
		t.Fatalf("decode run: %v", err)
	}

	req = authenticatedRequest(t, http.MethodGet, ts.URL+"/api/runs/"+run.ID+"/report?access_token=secret", nil)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("report request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected report 200, got %d", res.StatusCode)
	}

	output := logs.String()
	for _, want := range []string{"request_id=", "user_id=token-user", "run_id=" + run.ID, "status=200", "duration_ms="} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected log field %q in %s", want, output)
		}
	}
	for _, forbidden := range []string{"Authorization", "Bearer", "access_token", "secret"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("log output contains sensitive value %q: %s", forbidden, output)
		}
	}
}

func TestProxyUserOwnerIsolation(t *testing.T) {
	manager := sim.NewManager(store.NewMemory(), slog.Default())
	cfg := config.Default()
	cfg.AuthMode = config.AuthProxy
	cfg.AuthUserHeader = "X-Test-User"
	server := NewServerWithConfig(manager, slog.Default(), cfg)
	ts := httptest.NewServer(server.Routes())
	defer ts.Close()

	aliceRun, err := manager.CreateRunForOwner(context.Background(), "alice", "alice", sim.DefaultScenario())
	if err != nil {
		t.Fatalf("create alice run: %v", err)
	}
	bobRun, err := manager.CreateRunForOwner(context.Background(), "bob", "bob", sim.DefaultScenario())
	if err != nil {
		t.Fatalf("create bob run: %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/runs?limit=10", nil)
	if err != nil {
		t.Fatalf("new proxy request: %v", err)
	}
	req.Header.Set("X-Test-User", "alice")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("proxy list runs: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected proxy list 200, got %d", res.StatusCode)
	}
	var runs []model.Run
	if err := json.NewDecoder(res.Body).Decode(&runs); err != nil {
		t.Fatalf("decode proxy runs: %v", err)
	}
	if len(runs) != 1 || runs[0].ID != aliceRun.ID {
		t.Fatalf("expected alice to list only her run, got %+v", runs)
	}

	req, err = http.NewRequest(http.MethodGet, ts.URL+"/api/runs/"+bobRun.ID+"/report", nil)
	if err != nil {
		t.Fatalf("new proxy other request: %v", err)
	}
	req.Header.Set("X-Test-User", "alice")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("proxy get bob report: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("expected bob report 404 for alice, got %d", res.StatusCode)
	}
}

func TestAuthenticatedWebSocketUsesOneTimeTicket(t *testing.T) {
	cfg := config.Default()
	cfg.AuthMode = config.AuthToken
	cfg.AuthToken = "secret"
	server := NewServerWithConfig(sim.NewManager(store.NewMemory(), slog.Default()), slog.Default(), cfg)
	ts := httptest.NewServer(server.Routes())
	defer ts.Close()

	req := authenticatedRequest(t, http.MethodPost, ts.URL+"/api/runs", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create authenticated run: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected authenticated create 201, got %d", res.StatusCode)
	}
	var run model.Run
	if err := json.NewDecoder(res.Body).Decode(&run); err != nil {
		t.Fatalf("decode run: %v", err)
	}

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/runs/" + run.ID
	conn, wsRes, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if conn != nil {
		_ = conn.Close()
	}
	if err == nil {
		t.Fatal("expected websocket without ticket to fail")
	}
	if responseStatus(wsRes) != http.StatusUnauthorized {
		t.Fatalf("expected websocket without ticket 401, got %v", responseStatus(wsRes))
	}

	conn, wsRes, err = websocket.DefaultDialer.Dial(wsURL+"?access_token=secret", nil)
	if conn != nil {
		_ = conn.Close()
	}
	if err == nil {
		t.Fatal("expected websocket with long-lived query token to fail")
	}
	if responseStatus(wsRes) != http.StatusUnauthorized {
		t.Fatalf("expected websocket query token 401, got %v", responseStatus(wsRes))
	}

	ticket := requestWSTicket(t, ts.URL, run.ID)
	conn, wsRes, err = websocket.DefaultDialer.Dial(wsURL+"?ticket="+ticket, nil)
	if conn != nil {
		_ = conn.Close()
	}
	if err != nil {
		t.Fatalf("expected websocket ticket dial to succeed, status=%v err=%v", responseStatus(wsRes), err)
	}
	if responseStatus(wsRes) != http.StatusSwitchingProtocols {
		t.Fatalf("expected websocket 101, got %v", responseStatus(wsRes))
	}

	conn, wsRes, err = websocket.DefaultDialer.Dial(wsURL+"?ticket="+ticket, nil)
	if conn != nil {
		_ = conn.Close()
	}
	if err == nil {
		t.Fatal("expected reused websocket ticket to fail")
	}
	if responseStatus(wsRes) != http.StatusUnauthorized {
		t.Fatalf("expected reused ticket 401, got %v", responseStatus(wsRes))
	}
}

func TestWebSocketConnectsThroughMiddleware(t *testing.T) {
	server := NewServer(sim.NewManager(store.NewMemory(), slog.Default()), slog.Default())
	ts := httptest.NewServer(server.Routes())
	defer ts.Close()

	run := createTestRun(t, ts.URL)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/runs/" + run.ID
	conn, res, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if conn != nil {
		defer conn.Close()
	}
	if err != nil {
		t.Fatalf("expected websocket dial to succeed, status=%v err=%v", responseStatus(res), err)
	}
	if res == nil || res.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("expected websocket 101, got %v", responseStatus(res))
	}
}

func TestWebSocketRejectsUnknownOrigin(t *testing.T) {
	server := NewServer(sim.NewManager(store.NewMemory(), slog.Default()), slog.Default())
	ts := httptest.NewServer(server.Routes())
	defer ts.Close()

	run := createTestRun(t, ts.URL)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/runs/" + run.ID
	headers := http.Header{"Origin": []string{"http://evil.test"}}
	conn, res, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if conn != nil {
		_ = conn.Close()
	}
	if err == nil {
		t.Fatal("expected websocket dial to fail")
	}
	if res != nil && res.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 response, got %d", res.StatusCode)
	}
}

func createTestRun(t *testing.T, baseURL string) model.Run {
	t.Helper()
	res, err := http.Post(baseURL+"/api/runs", "application/json", bytes.NewBufferString(`{}`))
	if err != nil {
		t.Fatalf("create run request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", res.StatusCode)
	}
	var run model.Run
	if err := json.NewDecoder(res.Body).Decode(&run); err != nil {
		t.Fatalf("decode run: %v", err)
	}
	return run
}

func urlQueryEscape(value string) string {
	replacer := strings.NewReplacer(":", "%3A", "+", "%2B")
	return replacer.Replace(value)
}

func authenticatedRequest(t *testing.T, method, url string, body io.Reader) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("new authenticated request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	return req
}

func requestWSTicket(t *testing.T, baseURL, runID string) string {
	t.Helper()
	req := authenticatedRequest(t, http.MethodPost, baseURL+"/api/runs/"+runID+"/ws-ticket", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request websocket ticket: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected websocket ticket 201, got %d", res.StatusCode)
	}
	var payload struct {
		Ticket string `json:"ticket"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode websocket ticket: %v", err)
	}
	if payload.Ticket == "" {
		t.Fatal("expected websocket ticket")
	}
	return payload.Ticket
}

func responseStatus(res *http.Response) int {
	if res == nil {
		return 0
	}
	return res.StatusCode
}

func hasActorStat(stats []model.ActorStat, actorID string) bool {
	for _, stat := range stats {
		if stat.ActorID == actorID && stat.Count > 0 {
			return true
		}
	}
	return false
}
