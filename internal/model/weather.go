package model

import (
	"fmt"
	"math"
	"time"
)

// WeatherSample 气象采样。
type WeatherSample struct {
	ID         int64
	TakenAt    time.Time
	AirTempC   float64
	Humidity   float64 // 0~1
	WindMS     float64 // m/s
	RainfallMM float64
	EvapRateMM float64 // mm/day，由服务层计算填充
}

// Validate 校验气象样本字段。
func (w *WeatherSample) Validate() error {
	if w.Humidity < 0 || w.Humidity > 1 {
		return fmt.Errorf("%w: humidity out of [0,1]", ErrInvalidInput)
	}
	if w.WindMS < 0 || w.WindMS > 60 {
		return fmt.Errorf("%w: wind speed out of range", ErrInvalidInput)
	}
	if w.RainfallMM < 0 {
		return fmt.Errorf("%w: rainfall must be >= 0", ErrInvalidInput)
	}
	return nil
}

// EstimateEvapRate 经验蒸发率模型（mm/day）：
// 温度项 × (1-湿度) × 风速项；降雨当日折减。
func EstimateEvapRate(airTempC, humidity, windMS, rainfallMM float64) float64 {
	tempTerm := math.Max(0, airTempC-10) / 2.0
	dryTerm := 1 - humidity*0.8
	windTerm := 1 + math.Min(windMS, 10)/8.0
	rainCut := 1.0
	if rainfallMM > 5 {
		rainCut = 0.2
	} else if rainfallMM > 0 {
		rainCut = 0.6
	}
	return tempTerm * dryTerm * windTerm * rainCut
}

// RainRisk 降雨风险分级。
type RainRisk int

const (
	RainRiskNone RainRisk = iota
	RainRiskWatch
	RainRiskAlert
)

// AssessRainRisk 依近期降雨量与湿度评估风险等级。
func AssessRainRisk(samples []*WeatherSample) RainRisk {
	if len(samples) == 0 {
		return RainRiskNone
	}
	var rain, humidCount int
	for _, s := range samples {
		rain += int(s.RainfallMM)
		if s.Humidity > 0.85 {
			humidCount++
		}
	}
	switch {
	case rain >= 20:
		return RainRiskWatch
	case rain >= 5 && humidCount > len(samples)/2:
		return RainRiskWatch
	case rain >= 1 || humidCount > 0:
		return RainRiskNone
	default:
		return RainRiskNone
	}
}

// DilutionVolume 降雨稀释体积估算：降雨毫米数 × 受雨面积 → 立方米。
func DilutionVolume(rainfallMM, areaM2 float64) float64 {
	if rainfallMM <= 0 {
		return 0
	}
	return rainfallMM * areaM2 / 1000.0
}
