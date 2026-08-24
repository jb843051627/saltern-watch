package regression

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jb843051627/saltern-watch/internal/clock"
	"github.com/jb843051627/saltern-watch/internal/config"
	"github.com/jb843051627/saltern-watch/internal/model"
	"github.com/jb843051627/saltern-watch/internal/service"
	"github.com/jb843051627/saltern-watch/internal/store"
)

func TestBug02_BatchIngestRespectsCancel(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cfg := &config.Config{MaxBatchReads: 10, LocalZone: time.UTC}
	svc := service.New(st, clock.Real(), cfg)

	pond, err := svc.Ponds.Create("E1", 1000, 0, 7, 100)
	if err != nil {
		t.Fatal(err)
	}
	items := []*model.BrineReading{
		{PondID: pond.ID, Be: 5, TempC: 20, LevelCm: 95, Source: "manual"},
		{PondID: pond.ID, Be: 6, TempC: 20, LevelCm: 92, Source: "manual"},
		{PondID: pond.ID, Be: 7, TempC: 20, LevelCm: 90, Source: "manual"},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	n, err := svc.Brine.BatchIngest(ctx, items)
	if err == nil {
		t.Fatalf("cancelled batch must fail, got accepted=%d err=%v", n, err)
	}
	if n != 0 {
		t.Fatalf("accepted = %d, want 0", n)
	}
	big := make([]*model.BrineReading, 11)
	for i := range big {
		big[i] = &model.BrineReading{PondID: pond.ID, Be: 5, TempC: 20, LevelCm: 90, Source: "manual"}
	}
	if _, err := svc.Brine.BatchIngest(context.Background(), big); err == nil {
		t.Fatal("oversized batch must be rejected")
	}
}
