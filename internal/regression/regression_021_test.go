package regression

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/jb843051627/saltern-watch/internal/clock"
	"github.com/jb843051627/saltern-watch/internal/config"
	"github.com/jb843051627/saltern-watch/internal/model"
	"github.com/jb843051627/saltern-watch/internal/service"
	"github.com/jb843051627/saltern-watch/internal/store"
)

func TestBug21_OverdueScanScopeAndSyncError(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Unix(1750000000, 0)
	svc := service.New(st, clock.NewFake(now), &config.Config{LocalZone: time.UTC})

	open, err := svc.Maintenance.Plan(model.TargetPump, 1, "open task", now.Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	doneTask, err := svc.Maintenance.Plan(model.TargetPond, 2, "done task", now.Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Maintenance.Start(doneTask.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Maintenance.Complete(doneTask.ID); err != nil {
		t.Fatal(err)
	}
	n, err := svc.Maintenance.ScanOverdue()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("overdue scan counted %d tasks, want only the open one", n)
	}
	_ = open

	orphan, err := svc.Maintenance.Plan(model.TargetPump, 999, "orphan pump", now.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Maintenance.Start(orphan.ID); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Maintenance.Complete(orphan.ID)
	if err == nil {
		t.Fatal("completing task on missing pump must surface sync error")
	}
	if !errors.Is(err, model.ErrNotFound) && err.Error() == "" {
		t.Fatal("empty error")
	}
}
