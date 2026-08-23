package service

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/jb843051627/saltern-watch/internal/clock"
	"github.com/jb843051627/saltern-watch/internal/model"
	"github.com/jb843051627/saltern-watch/internal/store"
)

// QualityService 化验质检业务。
type QualityService struct {
	st    *store.DB
	clock clock.Clock
}

// RecordSample 记录化验并联动预测等级。
func (s *QualityService) RecordSample(crystID int64, naMg, sulfate, insoluble float64, analyst string) (*model.LabSample, error) {
	if _, err := s.st.GetCrystallizer(crystID); err != nil {
		return nil, err
	}
	l := &model.LabSample{
		CrystallizerID: crystID,
		TakenAt:        s.clock.Now(),
		NaMgRatio:      naMg, SulfatePPM: sulfate,
		InsolublePPM: insoluble, Analyst: analyst,
	}
	if err := s.st.InsertLabSample(l); err != nil {
		return nil, err
	}
	_ = s.st.InsertEvent(model.NewEvent("api", "lab", l.ID, "record",
		fmt.Sprintf("cryst=%d purity=%.2f", crystID, l.Purity), s.clock.Now()))
	return l, nil
}

// PredictGrade 依最新化验预测收盐等级；无化验记录回退水分判级。
func (s *QualityService) PredictGrade(crystID int64, moisture float64) (model.HarvestGrade, error) {
	purity, err := s.st.LatestPurity(crystID)
	if err == model.ErrNotFound {
		return model.GradeOf(moisture, 0), nil
	}
	if err != nil {
		return "", err
	}
	sample := &model.LabSample{Purity: purity}
	return sample.PredictedGrade(moisture), nil
}

// PurityTrend 品位趋势（时间正序），用于报表与告警。
func (s *QualityService) PurityTrend(crystID int64, limit int) ([]float64, error) {
	samples, err := s.st.ListLabSamplesByCryst(crystID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]float64, 0, len(samples))
	for i := len(samples) - 1; i >= 0; i-- {
		out = append(out, samples[i].Purity)
	}
	return out, nil
}

// PuritySlope 品位斜率（最近两次化验差/天）；样本不足返回 ErrNotFound。
func (s *QualityService) PuritySlope(crystID int64) (float64, error) {
	samples, err := s.st.ListLabSamplesByCryst(crystID, 2)
	if err != nil {
		return 0, err
	}
	if len(samples) < 2 {
		return 0, model.ErrNotFound
	}
	newest := samples[0]
	older := samples[1]
	days := newest.TakenAt.Sub(older.TakenAt).Hours() / 24.0
	if days <= 0 {
		days = 0.5
	}
	return (newest.Purity - older.Purity) / days, nil
}

// DeterioratingCrysts 品位下滑超阈值的结晶池列表。
func (s *QualityService) DeterioratingCrysts(thresholdPerDay float64) ([]int64, error) {
	crysts, err := s.st.ListCrystallizers()
	if err != nil {
		return nil, err
	}
	var out []int64
	for _, c := range crysts {
		slope, err := s.PuritySlope(c.ID)
		if err == model.ErrNotFound {
			continue
		}
		if err != nil {
			return nil, err
		}
		if slope < -math.Abs(thresholdPerDay) {
			out = append(out, c.ID)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// SensorService 传感器台账与校准。
type SensorService struct {
	st    *store.DB
	clock clock.Clock
}

// Register 登记传感器。
func (s *SensorService) Register(pondID int64, kind, modelName string) (*model.Sensor, error) {
	sen := &model.Sensor{PondID: pondID, Kind: kind, Model: modelName, Active: true}
	if err := s.st.InsertSensor(sen); err != nil {
		return nil, err
	}
	return sen, nil
}

// Calibrate 校准：记录历史并更新传感器偏移，返回偏移量。
func (s *SensorService) Calibrate(sensorID int64, referenceBe, rawBe float64) (*model.CalibrationRecord, error) {
	if _, err := s.st.GetSensor(sensorID); err != nil {
		return nil, err
	}
	rec := &model.CalibrationRecord{
		SensorID: sensorID, ReferenceBe: referenceBe, RawBe: rawBe,
	}
	offset := rec.ComputeOffset()
	if math.Abs(offset) > 5 {
		return nil, fmt.Errorf("%w: calibration offset %.2f implausible", model.ErrInvalidInput, offset)
	}
	if err := s.st.InsertCalibration(rec); err != nil {
		return nil, err
	}
	if err := s.st.SaveSensorOffset(sensorID, offset); err != nil {
		return nil, err
	}
	return rec, nil
}

// ApplyOffset 应用传感器偏移（仅活跃 be 类传感器）。
func (s *SensorService) ApplyOffset(sensorID int64, rawBe float64) (float64, error) {
	sen, err := s.st.GetSensor(sensorID)
	if err != nil {
		return rawBe, err
	}
	if !sen.Active || sen.Kind != "be" {
		return rawBe, nil
	}
	out := rawBe + sen.OffsetBe
	if out < model.BeMin || out > model.BeMax {
		return rawBe, fmt.Errorf("%w: calibrated Be %.2f out of range", model.ErrInvalidInput, out)
	}
	return out, nil
}

// SensorsByPond 池关联传感器列表（简化：全量过滤）。
func (s *SensorService) SensorsByPond(pondID int64) ([]*model.Sensor, error) {
	all, err := s.allSensors()
	if err != nil {
		return nil, err
	}
	var out []*model.Sensor
	for _, sen := range all {
		if sen.PondID == pondID {
			out = append(out, sen)
		}
	}
	return out, nil
}

func (s *SensorService) allSensors() ([]*model.Sensor, error) {
	rows, err := s.st.SQL().Query(`SELECT id,pond_id,kind,model,active,offset_be,updated FROM sensors ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("store: list sensors: %w", err)
	}
	defer rows.Close()
	var out []*model.Sensor
	for rows.Next() {
		var sen model.Sensor
		var updated int64
		if err := rows.Scan(&sen.ID, &sen.PondID, &sen.Kind, &sen.Model, &sen.Active, &sen.OffsetBe, &updated); err != nil {
			return nil, fmt.Errorf("store: scan sensor: %w", err)
		}
		sen.Updated = time.Unix(updated, 0)
		out = append(out, &sen)
	}
	return out, rows.Err()
}
