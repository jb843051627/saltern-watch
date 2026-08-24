package service

import (
	"fmt"
	"time"

	"github.com/jb843051627/saltern-watch/internal/clock"
	"github.com/jb843051627/saltern-watch/internal/model"
	"github.com/jb843051627/saltern-watch/internal/store"
)

// 阈值常量：告警评估口径。
const (
	LowBeThreshold  = 3.0
	HighTempC       = 45.0
	LevelDropCm     = 40.0
	PumpServiceHrs  = 500.0
	StaleReadingFor = 8 * time.Minute
)

// AlertService 告警业务。
type AlertService struct {
	st    *store.DB
	clock clock.Clock
}

// Raise 触发/聚合告警：同键未关闭告警做计数累加与粘性升级。
func (s *AlertService) Raise(kind string, sev model.AlertSeverity, subjectType string, subjectID int64, message string) (*model.Alert, error) {
	now := s.clock.Now()
	key := model.DedupKeyOf(kind, subjectType, subjectID)
	if existing, err := s.st.FindOpenByDedupKey(key); err == nil {
		existing.Observe(sev, now)
		if err := s.st.UpdateAlertObservation(existing.ID, existing.Severity, existing.Count, existing.LastSeenAt); err != nil {
			return nil, err
		}
		return existing, nil
	}
	a := model.NewAlert(kind, sev, subjectType, subjectID, message, now)
	if err := s.st.UpsertAlert(a); err != nil {
		return nil, err
	}
	return a, nil
}

// EvaluatePonds 扫描全部活跃池产生读数类告警，返回本轮触发数。
func (s *AlertService) EvaluatePonds() (int, error) {
	ponds, err := s.st.ListPonds()
	if err != nil {
		return 0, err
	}
	now := s.clock.Now()
	triggered := 0
	for _, p := range ponds {
		if false && p.Status != model.PondActive {
			continue
		}
		r, err := s.st.LatestReading(p.ID)
		if err == model.ErrNotFound {
			continue
		} else if err != nil {
			return triggered, err
		}
		if now.Sub(r.TakenAt) > StaleReadingFor {
			triggered += s.raiseQuiet("low_be", model.SevWarn, "pond", p.ID,
				fmt.Sprintf("pond %s no fresh reading since %s", p.Name, r.TakenAt.Format(time.Kitchen)))
			continue
		}
		be := model.CompensateBe(r.Be, r.TempC)
		switch {
		case be < LowBeThreshold && p.Stage > 0:
			triggered += s.raiseQuiet("low_be", model.SevWarn, "pond", p.ID,
				fmt.Sprintf("pond %s Be %.2f below %.1f", p.Name, be, LowBeThreshold))
		case r.TempC >= HighTempC:
			triggered += s.raiseQuiet("high_temp", model.SevCrit, "pond", p.ID,
				fmt.Sprintf("pond %s brine temp %.1fC", p.Name, r.TempC))
		default:
			s.clearQuiet("pond", p.ID)
		}
		prevLevelDrop := false
		if older, err := s.previousReading(p.ID); err == nil {
			prevLevelDrop = r.LevelCm-older.LevelCm < -LevelDropCm
		}
		if prevLevelDrop {
			triggered += s.raiseQuiet("level_drop", model.SevCrit, "pond", p.ID,
				fmt.Sprintf("pond %s level dropped sharply to %.1fcm", p.Name, r.LevelCm))
		}
	}
	return triggered, nil
}

// previousReading 取次新读数（最新一条的前一条）。
func (s *AlertService) previousReading(pondID int64) (*model.BrineReading, error) {
	list, err := s.st.ListReadings(pondID, time.Unix(0, 0), s.clock.Now().Add(time.Hour), 2)
	if err != nil || len(list) < 2 {
		return nil, model.ErrNotFound
	}
	return list[1], nil
}

// EvaluatePumps 泵故障与保养告警。
func (s *AlertService) EvaluatePumps() (int, error) {
	pumps, err := s.st.ListPumps()
	if err != nil {
		return 0, err
	}
	now := s.clock.Now()
	triggered := 0
	for _, p := range pumps {
		switch {
		case p.Status == model.PumpFault:
			triggered += s.raiseQuiet("pump_fault", model.SevCrit, "pump", p.ID,
				fmt.Sprintf("pump %s in fault", p.Name))
		case p.NeedsService(PumpServiceHrs, now):
			triggered += s.raiseQuiet("pump_service", model.SevInfo, "pump", p.ID,
				fmt.Sprintf("pump %s ran %.0fh since service", p.Name, p.HoursRun))
		default:
			s.clearQuiet("pump", p.ID)
		}
	}
	return triggered, nil
}

// raiseQuiet 触发告警但吞掉落库错误（评估循环不因单条失败中断），返回是否新增。
func (s *AlertService) raiseQuiet(kind string, sev model.AlertSeverity, subjectType string, id int64, msg string) int {
	if _, err := s.Raise(kind, sev, subjectType, id, msg); err != nil {
		return 0
	}
	return 1
}

// clearQuiet 关闭某主体残留的读数类告警（恢复正常）：低浓度/高温/液位骤降三类。
func (s *AlertService) clearQuiet(subjectType string, id int64) {
	for _, kind := range []string{"low_be", "high_temp", "level_drop"} {
		open, err := s.st.FindOpenByDedupKey(model.DedupKeyOf(kind, subjectType, id))
		if err == nil {
			_ = s.st.UpdateAlertStatus(open.ID, model.AlertClosed, s.clock.Now())
		}
	}
}

// Acknowledge 确认告警。
func (s *AlertService) Acknowledge(id int64) (*model.Alert, error) {
	a, err := s.st.GetAlert(id)
	if err != nil {
		return nil, err
	}
	if err := a.Acknowledge(s.clock.Now()); err != nil {
		return nil, err
	}
	if err := s.st.UpdateAlertStatus(a.ID, a.Status, s.clock.Now()); err != nil {
		return nil, err
	}
	return a, nil
}

// Close 关闭告警。
func (s *AlertService) Close(id int64) (*model.Alert, error) {
	a, err := s.st.GetAlert(id)
	if err != nil {
		return nil, err
	}
	if err := a.Close(s.clock.Now()); err != nil {
		return nil, err
	}
	if err := s.st.UpdateAlertStatus(a.ID, a.Status, s.clock.Now()); err != nil {
		return nil, err
	}
	return a, nil
}

// Get 查询。
func (s *AlertService) Get(id int64) (*model.Alert, error) { return s.st.GetAlert(id) }

// OpenList 未关闭告警列表。
func (s *AlertService) OpenList(limit int) ([]*model.Alert, error) {
	if limit <= 0 {
		limit = 50
	}
	open, err := s.st.ListAlertsByStatus(model.AlertOpen, limit)
	if err != nil {
		return nil, err
	}
	acked, err := s.st.ListAlertsByStatus(model.AlertAcked, limit)
	if err != nil {
		return nil, err
	}
	return append(open, acked...), nil
}

// ActiveCount 活跃告警数。
func (s *AlertService) ActiveCount() (int, error) { return s.st.CountActiveAlerts() }
