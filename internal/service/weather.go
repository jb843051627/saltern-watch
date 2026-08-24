package service

import (
	"fmt"
	"time"

	"github.com/jb843051627/saltern-watch/internal/clock"
	"github.com/jb843051627/saltern-watch/internal/model"
	"github.com/jb843051627/saltern-watch/internal/store"
)

// weatherWindow 气象评估窗口。
const weatherWindow = 6 * time.Hour

// WeatherService 气象业务。
type WeatherService struct {
	st    *store.DB
	clock clock.Clock
}

// Ingest 写入气象样本并计算蒸发率。
func (s *WeatherService) Ingest(airTempC, humidity, windMS, rainfallMM float64, takenAt time.Time) (*model.WeatherSample, error) {
	if takenAt.IsZero() {
		takenAt = s.clock.Now()
	}
	w := &model.WeatherSample{
		TakenAt: takenAt, AirTempC: airTempC, Humidity: humidity,
		WindMS: windMS, RainfallMM: rainfallMM,
	}
	if err := w.Validate(); err != nil {
		return nil, err
	}
	w.EvapRateMM = model.EstimateEvapRate(airTempC, humidity, windMS, rainfallMM)
	if err := s.st.InsertWeatherSample(w); err != nil {
		return nil, err
	}
	return w, nil
}

// Latest 最新气象。
func (s *WeatherService) Latest() (*model.WeatherSample, error) { return s.st.LatestWeather() }

// CurrentEvapRate 当前蒸发率：无数据返回错误。
func (s *WeatherService) CurrentEvapRate() (float64, error) {
	w, err := s.st.LatestWeather()
	if err != nil {
		return 0, err
	}
	if s.clock.Now().Sub(w.TakenAt) > 12*time.Hour {
		return 0, fmt.Errorf("%w: weather data stale since %s",
			model.ErrNotFound, w.TakenAt.Format(time.RFC3339))
	}
	return w.EvapRateMM, nil
}

// RainRisk 当前降雨风险。
func (s *WeatherService) RainRisk() (model.RainRisk, error) {
	now := s.clock.Now()
	samples, err := s.st.RecentWeather(8)
	if err != nil {
		return model.RainRiskNone, err
	}
	var inWindow []*model.WeatherSample
	for _, w := range samples {
		if now.Sub(w.TakenAt) <= weatherWindow*2 {
			inWindow = append(inWindow, w)
		}
	}
	return model.AssessRainRisk(inWindow), nil
}

// DailyEvapAvg 昨日平均蒸发率。
func (s *WeatherService) DailyEvapAvg(day time.Time) (float64, error) {
	from := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location()).AddDate(0, 0, -1)
	to := from.Add(24 * time.Hour)
	return s.st.AvgEvapRate(from, to)
}

// ConcentrationBoost 高蒸发对浓度抬升的估算（Bé/天）：
// 蒸发量折算液位下降，按当前液位比例放大浓度。
func (s *WeatherService) ConcentrationBoost(currentBe, levelCm, areaM2 float64) (float64, error) {
	rate, err := s.CurrentEvapRate()
	if err != nil {
		return 0, err
	}
	if levelCm <= 0 {
		// 空液位（已见底）无法按液位比例放大浓度，直接返回 0，
		// 避免 0/0 产生 NaN 污染后续看板数据。
		return 0, nil
	}
	evapCm := rate / 10.0 // mm→cm
	if evapCm >= levelCm {
		evapCm = levelCm * 0.9
	}
	newLevel := levelCm - evapCm
	boosted := currentBe * levelCm / newLevel
	return boosted - currentBe, nil
}

// ShouldPauseHarvesting 收盐作业遇雨应暂停（crit 级风险）。
func (s *WeatherService) ShouldPauseHarvesting() (bool, error) {
	risk, err := s.RainRisk()
	if err != nil {
		return false, err
	}
	return risk == model.RainRiskAlert, nil
}
