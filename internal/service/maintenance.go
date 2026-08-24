package service

import (
	"fmt"
	"time"

	"github.com/jb843051627/saltern-watch/internal/clock"
	"github.com/jb843051627/saltern-watch/internal/model"
	"github.com/jb843051627/saltern-watch/internal/store"
)

// MaintenanceService 维护任务业务。
type MaintenanceService struct {
	st    *store.DB
	clock clock.Clock
}

// Plan 新建维护任务。
func (s *MaintenanceService) Plan(targetType model.TargetKind, targetID int64, title string, dueAt time.Time) (*model.MaintenanceTask, error) {
	m := &model.MaintenanceTask{
		TargetType: targetType, TargetID: targetID,
		Title: title, Status: model.MaintPlanned,
		DueAt: dueAt,
	}
	if err := s.st.CreateMaintenanceTask(m); err != nil {
		return nil, err
	}
	_ = s.st.InsertEvent(model.NewEvent("api", "maintenance", m.ID, "plan",
		title, s.clock.Now()))
	return m, nil
}

// Start 开始任务。
func (s *MaintenanceService) Start(id int64) (*model.MaintenanceTask, error) {
	m, err := s.st.GetMaintenanceTask(id)
	if err != nil {
		return nil, err
	}
	if err := m.Start(); err != nil {
		return nil, err
	}
	if err := s.st.SaveMaintenanceTask(m); err != nil {
		return nil, err
	}
	_ = s.st.InsertEvent(model.NewEvent("api", "maintenance", id, "start",
		m.Title, s.clock.Now()))
	return m, nil
}

// Complete 完成任务；对象为泵时联动刷新保养时间。
func (s *MaintenanceService) Complete(id int64) (*model.MaintenanceTask, error) {
	m, err := s.st.GetMaintenanceTask(id)
	if err != nil {
		return nil, err
	}
	if err := m.Complete(s.clock.Now()); err != nil {
		return nil, err
	}
	if err := s.st.SaveMaintenanceTask(m); err != nil {
		return nil, err
	}
	if m.TargetType == model.TargetPump && m.Status == model.MaintDone {
		pumpSvc := &PumpService{st: s.st, clock: s.clock}
		_, _ = pumpSvc.Service(m.TargetID)
	}
	_ = s.st.InsertEvent(model.NewEvent("api", "maintenance", id, "complete",
		m.Title, s.clock.Now()))
	return m, nil
}

// Block 标记受阻。
func (s *MaintenanceService) Block(id int64, reason string) (*model.MaintenanceTask, error) {
	m, err := s.st.GetMaintenanceTask(id)
	if err != nil {
		return nil, err
	}
	if err := m.Block(reason); err != nil {
		return nil, err
	}
	if err := s.st.SaveMaintenanceTask(m); err != nil {
		return nil, err
	}
	_ = s.st.InsertEvent(model.NewEvent("api", "maintenance", id, "block",
		reason, s.clock.Now()))
	return m, nil
}

// Get 查询。
func (s *MaintenanceService) Get(id int64) (*model.MaintenanceTask, error) {
	return s.st.GetMaintenanceTask(id)
}

// ListByStatus 状态查询。
func (s *MaintenanceService) ListByStatus(status model.MaintenanceStatus) ([]*model.MaintenanceTask, error) {
	return s.st.ListMaintenanceTasksByStatus(status)
}

// ScanOverdue 逾期扫描并触发告警，返回逾期数。
func (s *MaintenanceService) ScanOverdue() (int, error) {
	tasks, err := s.st.OverdueMaintenanceTasks(s.clock.Now())
	if err != nil {
		return 0, err
	}
	alertSvc := &AlertService{st: s.st, clock: s.clock}
	for _, m := range tasks {
		subj := string(m.TargetType)
		_, _ = alertSvc.Raise("maintenance_overdue", model.SevWarn, subj, m.TargetID,
			fmt.Sprintf("task %q overdue since %s", m.Title, m.DueAt.Format("2006-01-02")))
	}
	return len(tasks), nil
}
