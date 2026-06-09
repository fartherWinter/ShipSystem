package runarchive

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"shipsim/internal/model"
	"shipsim/internal/store"
)

type RestoreOptions struct {
	AllowAppend bool
}

type RestoreSummary struct {
	RunID        string `json:"run_id"`
	Events       int    `json:"events"`
	Snapshots    int    `json:"snapshots"`
	Annotations  int    `json:"annotations"`
	AuditLogs    int    `json:"audit_logs"`
	TrainingOnly bool   `json:"training_only"`
}

type manifest struct {
	RunID        string `json:"run_id"`
	TrainingOnly bool   `json:"training_only"`
	SafetyNotice string `json:"safety_notice"`
}

type archiveFiles map[string][]byte

func Restore(ctx context.Context, st store.Store, archivePath string, opts RestoreOptions) (RestoreSummary, error) {
	files, err := loadArchiveFiles(archivePath)
	if err != nil {
		return RestoreSummary{}, err
	}
	var mf manifest
	if err := files.readJSON("manifest.json", &mf); err != nil {
		return RestoreSummary{}, fmt.Errorf("read manifest: %w", err)
	}
	if !mf.TrainingOnly {
		return RestoreSummary{}, errors.New("archive manifest is not marked training_only")
	}
	var run model.Run
	if err := files.readJSON("data/run.json", &run); err != nil {
		return RestoreSummary{}, fmt.Errorf("read run: %w", err)
	}
	if run.ID == "" {
		return RestoreSummary{}, errors.New("archive run is missing id")
	}
	if mf.RunID != "" && mf.RunID != run.ID {
		return RestoreSummary{}, fmt.Errorf("manifest run_id %q does not match data/run.json id %q", mf.RunID, run.ID)
	}
	if !opts.AllowAppend {
		counts, err := st.RunDataCounts(ctx, run.ID)
		if err != nil {
			return RestoreSummary{}, fmt.Errorf("check existing run data: %w", err)
		}
		if counts.Events+counts.TrackPoints+counts.Contacts+counts.Snapshots > 0 {
			return RestoreSummary{}, fmt.Errorf("run %s already has replay data; rerun with allow append only if duplicate data is intended", run.ID)
		}
	}
	if run.SafetyNotice == "" {
		run.SafetyNotice = mf.SafetyNotice
	}
	run.Restored = true
	if err := st.SaveRun(ctx, run); err != nil {
		return RestoreSummary{}, fmt.Errorf("restore run: %w", err)
	}

	summary := RestoreSummary{RunID: run.ID, TrainingOnly: mf.TrainingOnly}
	var events []model.SimEvent
	if err := files.readJSON("data/events.json", &events); err == nil {
		for _, event := range events {
			if event.RunID == "" {
				event.RunID = run.ID
			}
			if err := st.SaveEvent(ctx, event); err != nil {
				return RestoreSummary{}, fmt.Errorf("restore event: %w", err)
			}
			summary.Events++
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return RestoreSummary{}, fmt.Errorf("read events: %w", err)
	}

	snapshotFiles := files.list("data/snapshots/", ".json")
	for _, name := range snapshotFiles {
		var frames []model.SnapshotFrame
		if err := files.readJSON(name, &frames); err != nil {
			return RestoreSummary{}, fmt.Errorf("read snapshot page %s: %w", name, err)
		}
		for _, frame := range frames {
			if frame.RunID == "" {
				frame.RunID = run.ID
			}
			if err := st.SaveSnapshot(ctx, snapshotFromFrame(frame)); err != nil {
				return RestoreSummary{}, fmt.Errorf("restore snapshot: %w", err)
			}
			summary.Snapshots++
		}
	}

	var annotations []model.EventAnnotation
	if err := files.readJSON("data/annotations.json", &annotations); err == nil {
		for _, annotation := range annotations {
			if annotation.RunID == "" {
				annotation.RunID = run.ID
			}
			if _, err := st.SaveEventAnnotation(ctx, annotation); err != nil {
				return RestoreSummary{}, fmt.Errorf("restore annotation: %w", err)
			}
			summary.Annotations++
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return RestoreSummary{}, fmt.Errorf("read annotations: %w", err)
	}

	var auditLogs []model.AuditLog
	if err := files.readJSON("data/audit.json", &auditLogs); err == nil {
		for _, log := range auditLogs {
			if log.RunID == "" {
				log.RunID = run.ID
			}
			if err := st.SaveAuditLog(ctx, log); err != nil {
				return RestoreSummary{}, fmt.Errorf("restore audit log: %w", err)
			}
			summary.AuditLogs++
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return RestoreSummary{}, fmt.Errorf("read audit logs: %w", err)
	}

	return summary, nil
}

func loadArchiveFiles(path string) (archiveFiles, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return loadDirectoryArchive(path)
	}
	return loadZipArchive(path)
}

func loadDirectoryArchive(root string) (archiveFiles, error) {
	files := archiveFiles{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = body
		return nil
	})
	return files, err
}

func loadZipArchive(path string) (archiveFiles, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	files := archiveFiles{}
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		files[normalizeArchivePath(file.Name)] = body
	}
	return files, nil
}

func (f archiveFiles) readJSON(name string, out any) error {
	body, ok := f.find(name)
	if !ok {
		return os.ErrNotExist
	}
	if err := json.Unmarshal(body, out); err != nil {
		return err
	}
	return nil
}

func (f archiveFiles) find(name string) ([]byte, bool) {
	name = normalizeArchivePath(name)
	if body, ok := f[name]; ok {
		return body, true
	}
	suffix := "/" + name
	for path, body := range f {
		if strings.HasSuffix(path, suffix) {
			return body, true
		}
	}
	return nil, false
}

func (f archiveFiles) list(prefix, suffix string) []string {
	prefix = normalizeArchivePath(prefix)
	var names []string
	for name := range f {
		if (strings.HasPrefix(name, prefix) || strings.Contains(name, "/"+prefix)) && strings.HasSuffix(name, suffix) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func normalizeArchivePath(path string) string {
	return strings.TrimLeft(filepath.ToSlash(path), "/")
}

func snapshotFromFrame(frame model.SnapshotFrame) model.Snapshot {
	return model.Snapshot{
		RunID:      frame.RunID,
		Status:     frame.Status,
		Tick:       frame.Tick,
		Time:       frame.SampledAt,
		Tracks:     frame.Tracks,
		Contacts:   frame.Contacts,
		Notice:     frame.Notice,
		SnapshotHz: frame.SnapshotHz,
	}
}
