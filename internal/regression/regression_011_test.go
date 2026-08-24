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

func TestBug11_PumpServiceIntervalMath(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ck := clock.NewFake(time.Unix(1750000000, 0))
	svc := service.New(st, ck, &config.Config{LocalZone: time.UTC})

	p, err := svc.Pumps.Create("P-A", 60)
	if err != nil {
		t.Fatal(err)
	}
	p.HoursRun = 600
	if err := st.SavePump(p); err != nil {
		t.Fatal(err)
	}
	list, err := svc.Pumps.NeedsServiceList()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("pump with 600h must need service, got %d entries", len(list))
	}
	served, err := svc.Pumps.Service(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if served.HoursRun != 0 {
		t.Fatalf("service must reset hours, got %.1f", served.HoursRun)
	}
	fresh, err := svc.Pumps.Get(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.NeedsService(500, ck.Now()) {
		t.Fatal("freshly serviced pump must not need service")
	}
	_ = model.PumpStopped
}
