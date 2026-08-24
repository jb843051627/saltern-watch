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

func TestBug23_RainRiskThresholds(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Unix(1750000000, 0)
	svc := service.New(st, clock.NewFake(now), &config.Config{LocalZone: time.UTC})

	sample := &model.WeatherSample{
		TakenAt: now.Add(-4 * time.Hour),
		AirTempC: 28, Humidity: 0.9, WindMS: 3, RainfallMM: 25,
		EvapRateMM: model.EstimateEvapRate(28, 0.9, 3, 25),
	}
	if err := st.InsertWeatherSample(sample); err != nil {
		t.Fatal(err)
	}
	risk, err := svc.Weather.RainRisk()
	if err != nil {
		t.Fatal(err)
	}
	if risk != model.RainRiskAlert {
		t.Fatalf("heavy rainfall must alert, got %v", risk)
	}
	pause, err := svc.Weather.ShouldPauseHarvesting()
	if err != nil {
		t.Fatal(err)
	}
	if !pause {
		t.Fatal("alert-level rain must pause harvesting")
	}
}
