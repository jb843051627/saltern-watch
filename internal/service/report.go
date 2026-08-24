package service

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"github.com/jb843051627/saltern-watch/internal/clock"
	"github.com/jb843051627/saltern-watch/internal/model"
	"github.com/jb843051627/saltern-watch/internal/store"
)

// ReportService 报表导出业务。
type ReportService struct {
	st    *store.DB
	clock clock.Clock
	zone  *time.Location // 导出时间统一换算到场站时区
}

// DailyCSV 生成昨日生产日报 CSV：
// 各池蒸发损失/平均浓度 + 收盐批次明细 + 气象均值。
func (s *ReportService) DailyCSV(day time.Time) ([]byte, error) {
	day = day.In(s.zone)
	from := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, s.zone)
	to := from.Add(24 * time.Hour)

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write([]string{"section", "subject", "metric", "value"}); err != nil {
		return nil, err
	}
	fmtz := func(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

	ponds, err := s.st.ListPonds()
	if err != nil {
		return nil, err
	}
	for _, p := range ponds {
		readings, err := s.st.ListReadings(p.ID, from, to, 500)
		if err != nil || len(readings) == 0 {
			continue
		}
		first := readings[len(readings)-1]
		last := readings[0]
		loss := model.EvapLossM3(first.LevelCm, last.LevelCm, p.AreaM2)
		_ = w.Write([]string{"pond", p.Name, "evap_loss_m3", fmtz(loss)})
		avg, err := s.st.AverageBe(p.ID, from, to)
		if err == nil {
			_ = w.Write([]string{"pond", p.Name, "avg_be", fmtz(avg)})
		}
	}
	batches, err := s.st.ListHarvestBatches(from, to)
	if err != nil {
		return nil, err
	}
	for _, b := range batches {
		status := string(b.Status)
		grade := string(b.Grade)
		_ = w.Write([]string{"harvest", fmt.Sprintf("batch-%d", b.ID), "status_tons_grade",
			status + "," + fmtz(b.Tons) + "," + grade})
	}
	evap, err := s.st.AvgEvapRate(from, to)
	if err == nil {
		_ = w.Write([]string{"weather", "daily", "avg_evap_mm", fmtz(evap)})
	} else if err != model.ErrNotFound {
		return nil, err
	}
	rejected, err := s.st.CountRejected(from, to)
	if err != nil {
		return nil, err
	}
	_ = w.Write([]string{"quality", "readings", "rejected_count", strconv.Itoa(rejected)})

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("report: flush csv: %w", err)
	}
	return buf.Bytes(), nil
}

// FormatStamp 以场站时区格式化时间戳（导出列使用）。
func (s *ReportService) FormatStamp(t time.Time) string {
	return t.In(s.zone).Format("2006-01-02 15:04:05 -0700")
}

// Zone 返回场站时区。
func (s *ReportService) Zone() *time.Location { return s.zone }
