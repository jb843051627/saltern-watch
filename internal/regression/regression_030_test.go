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

func TestBug30_YieldEstimateMaturityCap(t *testing.T) {
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
	if _, err := svc.Crystallizers.FillBrine(cryst.ID, 90, 0.97); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Crystallizers.Transition(cryst.ID, model.CrystRipening); err != nil {
		t.Fatal(err)
	}
	ck.Advance(200 * time.Hour)
	if _, err := svc.Crystallizers.Transition(cryst.ID, model.CrystHarvestReady); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Crystallizers.Get(cryst.ID)
	if err != nil {
		t.Fatal(err)
	}
	yield := svc.Crystallizers.EstimateYield(got)
	maxYield := got.CapacityTons * (got.FilledTons / got.CapacityTons) * 0.32
	if yield > maxYield+0.01 {
		t.Fatalf("yield %.2f exceeds maturity-capped estimate %.2f", yield, maxYield)
	}
	fresh, err := svc.Crystallizers.Create("C2", 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Crystallizers.FillBrine(fresh.ID, 50, 0.9); err != nil {
		t.Fatal(err)
	}
	f2, _ := svc.Crystallizers.Get(fresh.ID)
	y2 := svc.Crystallizers.EstimateYield(f2)
	if y2 <= 0 || y2 > 16.01 {
		t.Fatalf("filling-pool estimate %.2f outside (0,16]", y2)
	}
}
