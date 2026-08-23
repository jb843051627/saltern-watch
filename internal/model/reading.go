package model

import (
	"fmt"
	"time"
)

// ReadingFlag 读数质量标记。
type ReadingFlag string

const (
	FlagOK       ReadingFlag = "ok"
	FlagSuspect  ReadingFlag = "suspect"
	FlagRejected ReadingFlag = "rejected"
)

// BrineReading 卤水读数。
type BrineReading struct {
	ID         int64
	PondID     int64
	TakenAt    time.Time
	Be         float64 // 波美度浓度
	TempC      float64 // 卤温
	LevelCm    float64 // 液位
	Source     string  // manual / sensor:<id>
	Flag       ReadingFlag
	RecordedAt time.Time
}

// BeRange 合法浓度区间（Bé）。
const (
	BeMin = 0.5
	BeMax = 29.0
)

// TempRange 合法卤温区间（摄氏度）。
const (
	TempMin = -2.0
	TempMax = 55.0
)

// ValidateReading 校验读数字段，返回带语义的错误。
func ValidateReading(r *BrineReading) error {
	if r.PondID <= 0 {
		return fmt.Errorf("%w: pond id required", ErrInvalidInput)
	}
	if r.Be < BeMin || r.Be > BeMax {
		return fmt.Errorf("%w: Be %.2f out of [%.1f,%.1f]", ErrInvalidInput, r.Be, BeMin, BeMax)
	}
	if r.TempC < TempMin || r.TempC > TempMax {
		return fmt.Errorf("%w: temp %.1f out of range", ErrInvalidInput, r.TempC)
	}
	if r.LevelCm < 0 || r.LevelCm > 500 {
		return fmt.Errorf("%w: level %.1f out of range", ErrInvalidInput, r.LevelCm)
	}
	if r.Source != "manual" && len(r.Source) <= 7 {
		return fmt.Errorf("%w: source must be manual or sensor:<id>", ErrInvalidInput)
	}
	if r.TakenAt.IsZero() {
		r.TakenAt = time.Now()
	}
	return nil
}

// CompensateBe 温度补偿：以 20°C 为基准，每偏差 1°C 浓度读数修正 0.03 Bé。
func CompensateBe(be, tempC float64) float64 {
	return be + (tempC-20.0)*0.03
}

// FlagOf 依浓度与液位突变幅度给出质量标记：突变大记 suspect，越界记 rejected。
func FlagOf(prev *BrineReading, cur *BrineReading) ReadingFlag {
	if prev.PondID != cur.PondID {
		return FlagOK
	}
	dBe := cur.Be - prev.Be
	dLevel := cur.LevelCm - prev.LevelCm
	switch {
	case dBe > 60 || dBe < -60:
		return FlagRejected
	case dLevel > 60 || dLevel < -60:
		return FlagSuspect
	default:
		return FlagOK
	}
}

// EvapLossM3 估算两次读数间的蒸发损失体积（简化模型）：
// 液位下降量 × 面积 / 100（cm→m），下降为负则无蒸发。
func EvapLossM3(prevLevel, curLevel, areaM2 float64) float64 {
	drop := prevLevel - curLevel
	if drop <= 0 {
		return 0
	}
	return areaM2 * drop / 100.0
}
