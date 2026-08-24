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

func TestBug28_PumpAssignmentDoubleBooking(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Unix(1750000000, 0)
	svc := service.New(st, clock.NewFake(now), &config.Config{LocalZone: time.UTC, MaxBatchReads: 10})

	src, err := svc.Ponds.Create("SRC", 1000, 1, 10, 200)
	if err != nil {
		t.Fatal(err)
	}
	dst, err := svc.Ponds.Create("DST", 1000, 2, 16, 100)
	if err != nil {
		t.Fatal(err)
	}
	pump, err := svc.Pumps.Create("P-A", 50)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Brine.Ingest(src.ID, 12, 20, 198, "manual", now); err != nil {
		t.Fatal(err)
	}
	faulty, err := svc.Pumps.Create("P-BAD", 40)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Pumps.MarkFault(faulty.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Pumps.AvailableFor(faulty.ID); !errors.Is(err, model.ErrPumpUnavailable) {
		t.Fatalf("faulty pump assignable: %v", err)
	}
	if _, err := svc.Transfers.Schedule(src.ID, dst.ID, pump.ID, 300, now.Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Pumps.AvailableFor(pump.ID); !errors.Is(err, model.ErrPumpUnavailable) {
		t.Fatalf("reserved pump double-bookable: %v", err)
	}
}
