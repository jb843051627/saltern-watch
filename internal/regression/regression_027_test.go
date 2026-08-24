package regression

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/jb843051627/saltern-watch/internal/clock"
	"github.com/jb843051627/saltern-watch/internal/config"
	"github.com/jb843051627/saltern-watch/internal/service"
	"github.com/jb843051627/saltern-watch/internal/store"
)

func TestBug27_PuritySlopeDegradationDetection(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	start := time.Unix(1750000000, 0)
	ck := clock.NewFake(start)
	svc := service.New(st, ck, &config.Config{LocalZone: time.UTC})

	cryst, err := svc.Crystallizers.Create("C1", 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Quality.RecordSample(cryst.ID, 14, 4, 30, "alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Quality.RecordSample(cryst.ID, 12, 8, 60, "alice"); err != nil {
		t.Fatal(err)
	}
	slope, err := svc.Quality.PuritySlope(cryst.ID)
	if err != nil {
		t.Fatal(err)
	}
	if math.IsInf(slope, 0) || math.IsNaN(slope) {
		t.Fatalf("same-time slope degenerated to %v (zero-day guard missing)", slope)
	}
	ck.Advance(24 * time.Hour)
	if _, err := svc.Quality.RecordSample(cryst.ID, 6, 20, 150, "bob"); err != nil {
		t.Fatal(err)
	}
	slope, err = svc.Quality.PuritySlope(cryst.ID)
	if err != nil {
		t.Fatal(err)
	}
	if math.IsInf(slope, 0) || math.IsNaN(slope) {
		t.Fatalf("slope degenerated to %v (zero-day guard missing)", slope)
	}
	bad, err := svc.Quality.DeterioratingCrysts(0.1)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, id := range bad {
		if id == cryst.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("deteriorating purity (slope %.3f/day) not flagged", slope)
	}
}
