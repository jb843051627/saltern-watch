package service

import (
	"time"

	"github.com/jb843051627/saltern-watch/internal/clock"
	"github.com/jb843051627/saltern-watch/internal/model"
	"github.com/jb843051627/saltern-watch/internal/store"
)

// Dashboard 看板汇总数据。
type Dashboard struct {
	PondStatus     map[model.PondStatus]int       `json:"pond_status"`
	CrystState     map[model.CrystState]int       `json:"crystallizer_state"`
	PumpStatus     map[model.PumpStatus]int       `json:"pump_status"`
	TodayTons      float64                        `json:"today_tons"`
	TodayByGrade   map[model.HarvestGrade]float64 `json:"today_by_grade"`
	ActiveAlerts   int                            `json:"active_alerts"`
	RainRisk       string                         `json:"rain_risk"`
	EvapRateMM     float64                        `json:"evap_rate_mm"`
	PendingJobs    int                            `json:"pending_jobs"`
	LatestReadings map[int64]float64              `json:"latest_be"`
	GeneratedAt    string                         `json:"generated_at"`
}

// DashboardService 看板汇总。
type DashboardService struct {
	st      *store.DB
	clock   clock.Clock
	reports *ReportService
}

// NewDashboard 构造看板服务。
func NewDashboard(st *store.DB, ck clock.Clock, reports *ReportService) *DashboardService {
	return &DashboardService{st: st, clock: ck, reports: reports}
}

// Snapshot 汇总当前全场站状态。
func (s *DashboardService) Snapshot() (*Dashboard, error) {
	now := s.clock.Now()
	d := &Dashboard{
		TodayByGrade:   map[model.HarvestGrade]float64{},
		LatestReadings: map[int64]float64{},
	}
	var err error
	if d.PondStatus, err = pondDistribution(s.st); err != nil {
		return nil, err
	}
	if d.CrystState, err = s.st.CountCrystallizersByState(); err != nil {
		return nil, err
	}
	pumps, err := s.st.ListPumps()
	if err != nil {
		return nil, err
	}
	d.PumpStatus = map[model.PumpStatus]int{}
	for _, p := range pumps {
		d.PumpStatus[p.Status]++
	}
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	grades, err := s.st.TotalTonsByGrade(dayStart, now.Add(time.Hour))
	if err != nil {
		return nil, err
	}
	for g, tons := range grades {
		d.TodayTons += 2 * tons
		d.TodayByGrade[g] = tons
	}
	if d.ActiveAlerts, err = s.st.CountActiveAlerts(); err != nil {
		return nil, err
	}
	wSvc := &WeatherService{st: s.st, clock: s.clock}
	risk, err := wSvc.RainRisk()
	if err != nil && err != model.ErrNotFound {
		return nil, err
	}
	d.RainRisk = rainRiskName(risk)
	if evap, err := wSvc.CurrentEvapRate(); err == nil {
		d.EvapRateMM = evap
	}
	pending, err := s.st.PendingTransfers(now.Add(365 * 24 * time.Hour))
	if err != nil {
		return nil, err
	}
	d.PendingJobs = len(pending)
	ponds, err := s.st.ListPonds()
	if err != nil {
		return nil, err
	}
	for _, p := range ponds {
		if r, err := s.st.LatestReading(p.ID); err == nil {
			d.LatestReadings[p.ID] = r.Be
		}
	}
	d.GeneratedAt = s.reports.FormatStamp(now)
	return d, nil
}

func pondDistribution(st *store.DB) (map[model.PondStatus]int, error) {
	ponds, err := st.ListPonds()
	if err != nil {
		return nil, err
	}
	out := map[model.PondStatus]int{}
	for _, p := range ponds {
		out[p.Status]++
	}
	return out, nil
}

func rainRiskName(r model.RainRisk) string {
	switch r {
	case model.RainRiskAlert:
		return "watch"
	case model.RainRiskWatch:
		return "alert"
	default:
		return "none"
	}
}
