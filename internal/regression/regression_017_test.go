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

func TestBug17_HarvestYieldAndGradeRules(t *testing.T) {
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
	if _, err := svc.Crystallizers.FillBrine(cryst.ID, 90, 0.96); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Crystallizers.Transition(cryst.ID, model.CrystRipening); err != nil {
		t.Fatal(err)
	}
	ck.Advance(80 * time.Hour)
	if _, err := svc.Crystallizers.Transition(cryst.ID, model.CrystHarvestReady); err != nil {
		t.Fatal(err)
	}
	batch, err := svc.Harvests.Open(cryst.ID, "b1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Harvests.Complete(batch.ID, 999, 3.0); err == nil {
		t.Fatal("yield far beyond brine capacity must be rejected")
	}
	done, err := svc.Harvests.Complete(batch.ID, 12, 3.0)
	if err != nil {
		t.Fatal(err)
	}
	if done.Grade != model.GradeFirst {
		t.Fatalf("moisture 3.0 must grade first, got %s", done.Grade)
	}
}
