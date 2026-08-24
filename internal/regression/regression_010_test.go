package regression

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jb843051627/saltern-watch/internal/clock"
	"github.com/jb843051627/saltern-watch/internal/config"
	"github.com/jb843051627/saltern-watch/internal/model"
	"github.com/jb843051627/saltern-watch/internal/service"
	"github.com/jb843051627/saltern-watch/internal/store"
)

func TestBug10_DailyCSVTimezoneWindow(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	zone := time.FixedZone("CST", 8*3600)
	cfg := &config.Config{LocalZone: zone}
	ck := clock.NewFake(time.Date(2026, 8, 22, 12, 0, 0, 0, zone))
	svc := service.New(st, ck, cfg)

	cryst, err := svc.Crystallizers.Create("C1", 100)
	if err != nil {
		t.Fatal(err)
	}
	batch := &model.HarvestBatch{
		CrystallizerID: cryst.ID, Status: model.BatchCompleted,
		Tons: 12, Moisture: 2.5, Grade: model.GradeSuper,
		OpenedAt: time.Date(2026, 8, 22, 1, 0, 0, 0, zone),
	}
	completed := batch.OpenedAt.Add(2 * time.Hour)
	batch.CompletedAt = &completed
	if err := st.CreateHarvestBatch(batch); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveHarvestBatch(batch); err != nil {
		t.Fatal(err)
	}

	data, err := svc.Reports.DailyCSV(time.Date(2026, 8, 22, 12, 0, 0, 0, zone))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("batch")) {
		t.Fatalf("daily csv missing harvest rows:\n%s", data)
	}
	stamp := svc.Reports.FormatStamp(time.Date(2026, 8, 22, 8, 0, 0, 0, time.UTC))
	if !strings.HasPrefix(stamp, "2026-08-22 16:") || !strings.HasSuffix(stamp, "+0800") {
		t.Fatalf("FormatStamp ignored station zone: %s", stamp)
	}
}
