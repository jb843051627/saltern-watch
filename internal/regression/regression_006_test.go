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

func TestBug06_CrystallizerStateMachineEdges(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ck := clock.NewFake(time.Unix(1750000000, 0))
	svc := service.New(st, ck, &config.Config{LocalZone: time.UTC})

	cryst, err := svc.Crystallizers.Create("C1", 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Crystallizers.FillBrine(cryst.ID, 40, 0.9); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Crystallizers.Transition(cryst.ID, model.CrystRipening); err == nil {
		t.Fatal("fill 40% must not enter ripening")
	}
	if _, err := svc.Crystallizers.FillBrine(cryst.ID, 55, 0.97); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Crystallizers.Transition(cryst.ID, model.CrystRipening); err != nil {
		t.Fatal(err)
	}
	ck.Advance(80 * time.Hour)
	if _, err := svc.Crystallizers.Transition(cryst.ID, model.CrystHarvestReady); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Crystallizers.Transition(cryst.ID, model.CrystHarvesting); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Crystallizers.Transition(cryst.ID, model.CrystEmpty)
	if err != nil {
		t.Fatalf("harvesting -> empty must be allowed, got %v", err)
	}
	if got.FilledTons != 0 {
		t.Fatalf("empty pool must reset filled tons, got %.2f", got.FilledTons)
	}
}
