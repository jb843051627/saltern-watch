package regression

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/jb843051627/saltern-watch/internal/clock"
	"github.com/jb843051627/saltern-watch/internal/config"
	"github.com/jb843051627/saltern-watch/internal/model"
	"github.com/jb843051627/saltern-watch/internal/service"
	"github.com/jb843051627/saltern-watch/internal/store"
)

func TestBug13_ConcentrationBoostGuards(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Unix(1750000000, 0)
	svc := service.New(st, clock.NewFake(now), &config.Config{LocalZone: time.UTC})

	if _, err := svc.Weather.Ingest(30, 0.5, 4, 0, now); err != nil {
		t.Fatal(err)
	}
	boost, err := svc.Weather.ConcentrationBoost(20, 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if boost != 0 || math.IsNaN(boost) {
		t.Fatalf("boost on dry pond = %v, want 0", boost)
	}
	rate := model.EstimateEvapRate(30, 0.5, 4, 10)
	want := ((30 - 10) / 2.0) * (1 - 0.5*0.8) * (1 + 4.0/8.0) * 0.2
	if math.Abs(rate-want) > 1e-9 {
		t.Fatalf("heavy-rain evap cut wrong: got %.4f want %.4f", rate, want)
	}
}
