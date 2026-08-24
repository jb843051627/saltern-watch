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

func TestBug19_RipeningPromotionGates(t *testing.T) {
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
	if _, err := svc.Crystallizers.FillBrine(cryst.ID, 90, 0.90); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Crystallizers.Transition(cryst.ID, model.CrystRipening); err != nil {
		t.Fatal(err)
	}
	ck.Advance(25 * time.Hour)
	n, err := svc.Crystallizers.PromoteRipened()
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("under-ripe pool promoted (%d), salinity gate lost", n)
	}
	got, err := svc.Crystallizers.Get(cryst.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != model.CrystRipening {
		t.Fatalf("state = %s, want ripening", got.State)
	}
}
