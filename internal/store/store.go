package store

import (
	"context"
	"time"

	"shipsim/internal/model"
)

type Store interface {
	Name() string
	Ready(context.Context) (model.StoreStatus, error)
	SaveRun(context.Context, model.Run) error
	GetRun(context.Context, string) (model.Run, error)
	GetRunForOwner(context.Context, string, string) (model.Run, error)
	ListRuns(context.Context, int) ([]model.Run, error)
	ListRunsForOwner(context.Context, int, string) ([]model.Run, error)
	SaveEvent(context.Context, model.SimEvent) error
	ListEvents(context.Context, string, model.EventQuery) (model.EventPage, error)
	SaveSnapshot(context.Context, model.Snapshot) error
	ListSnapshots(context.Context, string, model.SnapshotQuery) ([]model.SnapshotFrame, error)
	NearestSnapshot(context.Context, string, time.Time) (model.SnapshotFrame, error)
	LatestSnapshot(context.Context, string) (model.SnapshotFrame, error)
	SnapshotRange(context.Context, string) (model.SnapshotRange, bool, error)
	ListTracks(context.Context, string) ([]model.Track, error)
	ListTrackPoints(context.Context, string, model.TrackPointQuery) ([]model.TrackPoint, error)
	ListZones(context.Context, string) ([]model.Zone, error)
	ListScenarioSummaries(context.Context) ([]model.ScenarioSummary, error)
	GetScenario(context.Context, string) (model.Scenario, error)
	PreviewPrune(context.Context, model.RetentionPolicy) (model.RetentionPreview, error)
	Prune(context.Context, model.RetentionPolicy) (model.RetentionResult, error)
	Close()
}
