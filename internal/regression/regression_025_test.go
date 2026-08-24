package regression

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/jb843051627/saltern-watch/internal/clock"
	"github.com/jb843051627/saltern-watch/internal/config"
	"github.com/jb843051627/saltern-watch/internal/model"
	"github.com/jb843051627/saltern-watch/internal/service"
	"github.com/jb843051627/saltern-watch/internal/store"
)

func TestBug25_AlertAggregationStickySeverity(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ck := clock.NewFake(time.Unix(1750000000, 0))
	svc := service.New(st, ck, &config.Config{LocalZone: time.UTC})

	pond, err := svc.Ponds.Create("E1", 1000, 0, 7, 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Alerts.Raise("low_be", model.SevInfo, "pond", pond.ID, "a"); err != nil {
		t.Fatal(err)
	}
	merged, err := svc.Alerts.Raise("low_be", model.SevCrit, "pond", pond.ID, "b")
	if err != nil {
		t.Fatal(err)
	}
	if merged.Severity != model.SevCrit {
		t.Fatalf("repeat observation must escalate to crit, got %s", merged.Severity)
	}
	if merged.Count < 2 {
		t.Fatalf("aggregated count = %d, want >=2", merged.Count)
	}
	final, err := svc.Alerts.Raise("low_be", model.SevInfo, "pond", pond.ID, "c")
	if err != nil {
		t.Fatal(err)
	}
	if final.Severity != model.SevCrit {
		t.Fatalf("severity downgraded to %s, want sticky crit", final.Severity)
	}
	if _, err := svc.Alerts.Close(final.ID); err != nil {
		t.Fatal(err)
	}
	active, err := svc.Alerts.ActiveCount()
	if err != nil {
		t.Fatal(err)
	}
	if active != 0 {
		t.Fatalf("closed alerts still counted active: %d", active)
	}
	_ = st
}
