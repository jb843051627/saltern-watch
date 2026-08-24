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

func TestBug26_DailyEvapLossBounds(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Unix(1750000000, 0)
	svc := service.New(st, clock.NewFake(now), &config.Config{LocalZone: time.UTC})

	pond, err := svc.Ponds.Create("E1", 1000, 0, 7, 100)
	if err != nil {
		t.Fatal(err)
	}
	panicked := false
	var lossErr error
	func() {
		defer func() {
			if recover() != nil {
				panicked = true
			}
		}()
		_, _, lossErr = svc.Ponds.DailyEvapLoss(now)
	}()
	if panicked {
		t.Fatal("daily evap loss on pond without readings caused a panic")
	}
	if lossErr != nil {
		t.Fatal(lossErr)
	}

	loss := model.EvapLossM3(100, 102, 1000)
	if loss != 0 {
		t.Fatalf("rising level reported as evaporation loss %.2f m3", loss)
	}
	_ = pond
}
