package service

import (
	"fmt"
	"time"

	"github.com/jb843051627/saltern-watch/internal/clock"
	"github.com/jb843051627/saltern-watch/internal/model"
	"github.com/jb843051627/saltern-watch/internal/store"
)

// serviceHours 保养阈值（运行小时）。
const serviceHours = 500.0

// PumpService 泵站业务。
type PumpService struct {
	st    *store.DB
	clock clock.Clock
}

// Create 新建泵站。
func (s *PumpService) Create(name string, capacityM3H float64) (*model.Pump, error) {
	p := &model.Pump{Name: name, CapacityM3H: capacityM3H, Status: model.PumpStopped}
	if err := s.st.CreatePump(p); err != nil {
		return nil, err
	}
	_ = s.st.InsertEvent(model.NewEvent("api", "pump", p.ID, "create",
		fmt.Sprintf("capacity=%.0f", capacityM3H), s.clock.Now()))
	return p, nil
}

// Get 查询。
func (s *PumpService) Get(id int64) (*model.Pump, error) { return s.st.GetPump(id) }

// List 列表。
func (s *PumpService) List() ([]*model.Pump, error) { return s.st.ListPumps() }

// MarkFault 标记故障。
func (s *PumpService) MarkFault(id int64) (*model.Pump, error) {
	p, err := s.st.GetPump(id)
	if err != nil {
		return nil, err
	}
	p.Status = model.PumpFault
	if err := s.st.SavePump(p); err != nil {
		return nil, err
	}
	_ = s.st.InsertEvent(model.NewEvent("api", "pump", id, "mark_fault",
		p.Name, s.clock.Now()))
	return p, nil
}

// Service 保养完成：清零运行时长并刷新保养时间。
func (s *PumpService) Service(id int64) (*model.Pump, error) {
	p, err := s.st.GetPump(id)
	if err != nil {
		return nil, err
	}
	p.HoursRun = 0
	p.LastServiceAt = s.clock.Now()
	if err := s.st.SavePump(p); err != nil {
		return nil, err
	}
	_ = s.st.InsertEvent(model.NewEvent("api", "pump", id, "service",
		p.Name, s.clock.Now()))
	return p, nil
}

// NeedsServiceList 待保养泵列表。
func (s *PumpService) NeedsServiceList() ([]*model.Pump, error) {
	all, err := s.st.ListPumps()
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	var out []*model.Pump
	for _, p := range all {
		if p.NeedsService(serviceHours, now) {
			out = append(out, p)
		}
	}
	return out, nil
}

// AvailableFor 检查泵是否可被任务指派：存在且非故障、无其他未完结任务占用。
func (s *PumpService) AvailableFor(pumpID int64) (*model.Pump, error) {
	p, err := s.st.GetPump(pumpID)
	if err != nil {
		return nil, err
	}
	if !p.Status.CanAssign() {
		return nil, fmt.Errorf("%w: pump %d in %s", model.ErrPumpUnavailable, pumpID, p.Status)
	}
	active, err := s.st.ListTransfersByStatus(model.TransferRunning, 50)
	if err != nil {
		return nil, err
	}
	for _, t := range active {
		if t.ID != 0 && t.PumpID == pumpID {
			return nil, fmt.Errorf("%w: pump %d busy with job %d",
				model.ErrPumpUnavailable, pumpID, t.ID)
		}
	}
	pending, err := s.st.PendingTransfers(s.clock.Now().Add(365 * 24 * time.Hour))
	if err != nil {
		return nil, err
	}
	for _, t := range pending {
		if t.PumpID == pumpID && t.Status == model.TransferPending {
			return nil, fmt.Errorf("%w: pump %d reserved by job %d",
				model.ErrPumpUnavailable, pumpID, t.ID)
		}
	}
	return p, nil
}

// AccumulateHours 累加运行时长（worker 执行完任务调用）。
func (s *PumpService) AccumulateHours(pumpID int64, hours float64) (*model.Pump, error) {
	p, err := s.st.GetPump(pumpID)
	if err != nil {
		return nil, err
	}
	if hours < 0 || hours > 24*30 {
		return nil, fmt.Errorf("%w: implausible hours %.2f", model.ErrInvalidInput, hours)
	}
	p.HoursRun += hours
	if p.HoursRun > 100000 {
		p.HoursRun = 100000
	}
	if err := s.st.SavePump(p); err != nil {
		return nil, err
	}
	return p, nil
}

// StatusCounts 泵状态统计。
func (s *PumpService) StatusCounts() (map[model.PumpStatus]int, error) {
	pumps, err := s.st.ListPumps()
	if err != nil {
		return nil, err
	}
	out := map[model.PumpStatus]int{}
	for _, p := range pumps {
		out[p.Status]++
	}
	return out, nil
}
