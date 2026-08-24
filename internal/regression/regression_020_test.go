package regression

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/jb843051627/saltern-watch/internal/clock"
	"github.com/jb843051627/saltern-watch/internal/config"
	"github.com/jb843051627/saltern-watch/internal/service"
	"github.com/jb843051627/saltern-watch/internal/store"
)

func TestBug20_StaleReadingWindowConsistency(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	base := time.Unix(1750000000, 0)
	svc := service.New(st, clock.NewFake(base.Add(2*time.Hour)), &config.Config{LocalZone: time.UTC, MaxBatchReads: 10})

	pond, err := svc.Ponds.Create("E1", 1000, 1, 11, 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Brine.Ingest(pond.ID, 12, 25, 98, "manual", base); err != nil {
		t.Fatal(err)
	}
	stale, err := svc.Brine.StalePonds(8 * time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) != 0 {
		t.Fatalf("reading only 2h old flagged stale: %+v", stale)
	}
	if _, err := svc.Alerts.EvaluatePonds(); err != nil {
		t.Fatal(err)
	}
	active, err := svc.Alerts.ActiveCount()
	if err != nil {
		t.Fatal(err)
	}
	if active != 0 {
		t.Fatalf("fresh-ish pond raised %d alerts via shrunken window", active)
	}
}
