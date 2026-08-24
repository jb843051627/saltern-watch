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

func TestBug22_GroupGapDetectionAndMissingGroup(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Unix(1750000000, 0)
	svc := service.New(st, clock.NewFake(now), &config.Config{LocalZone: time.UTC, MaxBatchReads: 10})

	up, err := svc.Ponds.Create("UP", 1000, 0, 7, 120)
	if err != nil {
		t.Fatal(err)
	}
	down, err := svc.Ponds.Create("DOWN", 1000, 1, 11, 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Brine.Ingest(up.ID, 5, 20, 118, "manual", now); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Brine.Ingest(down.ID, 8, 20, 98, "manual", now); err != nil {
		t.Fatal(err)
	}
	group, err := svc.PondGroups.Create("G1", []int64{up.ID, down.ID}, 20)
	if err != nil {
		t.Fatal(err)
	}
	gaps, err := svc.PondGroups.GradientGaps(group.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) != 1 {
		t.Fatalf("concentration inversion undetected: %d gaps", len(gaps))
	}
	panicked := false
	var balanceErr error
	func() {
		defer func() {
			if recover() != nil {
				panicked = true
			}
		}()
		_, balanceErr = svc.PondGroups.BalancePlan(999)
	}()
	if panicked {
		t.Fatal("missing pond group caused a nil dereference panic")
	}
	if balanceErr != model.ErrNotFound {
		t.Fatalf("missing group error = %v, want ErrNotFound", balanceErr)
	}
}
