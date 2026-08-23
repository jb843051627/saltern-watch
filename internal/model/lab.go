package model

import (
	"fmt"
	"time"
)

// LabSample 卤水化验样本（全分析）：钠镁比、硫酸根、不溶物决定结晶品质预测。
type LabSample struct {
	ID             int64
	CrystallizerID int64
	TakenAt        time.Time
	NaMgRatio      float64 // 钠镁比（≥8 优）
	SulfatePPM     float64 // 硫酸根 ppm（≤10 优）
	InsolublePPM   float64 // 不溶物 ppm（≤50 合格）
	Analyst        string
	Purity         float64 // 由 EvaluatePurity 计算 0~1
}

// Validate 化验数据校验。
func (l *LabSample) Validate() error {
	if l.CrystallizerID <= 0 {
		return fmt.Errorf("%w: crystallizer required", ErrInvalidInput)
	}
	if l.NaMgRatio < 0 || l.NaMgRatio > 100 {
		return fmt.Errorf("%w: NaMg ratio out of range", ErrInvalidInput)
	}
	if l.SulfatePPM < 0 || l.InsolublePPM < 0 {
		return fmt.Errorf("%w: negative concentration", ErrInvalidInput)
	}
	if l.Analyst == "" {
		return fmt.Errorf("%w: analyst required", ErrInvalidInput)
	}
	return nil
}

// EvaluatePurity 品位评分：
// 钠镁比 ≥12 满分线性下降；硫酸根与不溶物按上限折减。
func (l *LabSample) EvaluatePurity() float64 {
	ratioScore := l.NaMgRatio / 12.0
	if ratioScore > 1 {
		ratioScore = 1
	}
	sulfateScore := 1 - l.SulfatePPM/40.0
	if sulfateScore < 0 {
		sulfateScore = 0
	}
	insolScore := 1 - l.InsolublePPM/200.0
	if insolScore < 0 {
		insolScore = 0
	}
	l.Purity = 0.5*ratioScore + 0.3*sulfateScore + 0.2*insolScore
	if l.Purity < 0 {
		l.Purity = 0
	}
	return l.Purity
}

// PredictedGrade 依品位预测收盐等级。
func (l *LabSample) PredictedGrade(moisture float64) HarvestGrade {
	purity := l.Purity
	if purity == 0 {
		purity = l.EvaluatePurity()
	}
	switch {
	case purity >= 0.85 && moisture <= MoistureLimit[GradeSuper]:
		return GradeSuper
	case purity >= 0.65:
		return GradeFirst
	default:
		return GradeSecond
	}
}

// Sensor 传感器台账。
type Sensor struct {
	ID       int64
	PondID   int64
	Kind     string // be / temp / level
	Model    string
	Active   bool
	OffsetBe float64 // 当前校准偏移（Bé）
	Updated  time.Time
}

// Validate 台账校验。
func (s *Sensor) Validate() error {
	if s.PondID <= 0 {
		return fmt.Errorf("%w: pond required", ErrInvalidInput)
	}
	switch s.Kind {
	case "be", "temp", "level":
	default:
		return fmt.Errorf("%w: unknown sensor kind %q", ErrInvalidInput, s.Kind)
	}
	if s.Model == "" {
		return fmt.Errorf("%w: sensor model required", ErrInvalidInput)
	}
	return nil
}

// CalibrationRecord 校准记录。
type CalibrationRecord struct {
	ID          int64
	SensorID    int64
	ReferenceBe float64 // 标准液实测值
	RawBe       float64 // 传感器原始读值
	Offset      float64 // reference - raw
	CreatedAt   time.Time
}

// ComputeOffset 计算并返回偏移量。
func (c *CalibrationRecord) ComputeOffset() float64 {
	c.Offset = c.ReferenceBe - c.RawBe
	return c.Offset
}
