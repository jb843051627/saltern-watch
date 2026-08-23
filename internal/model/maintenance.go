package model

import (
	"fmt"
	"time"
)

// MaintenanceStatus 维护任务状态。
type MaintenanceStatus string

const (
	MaintPlanned    MaintenanceStatus = "planned"
	MaintInProgress MaintenanceStatus = "in_progress"
	MaintDone       MaintenanceStatus = "done"
	MaintBlocked    MaintenanceStatus = "blocked"
)

// TargetKind 维护对象类型。
type TargetKind string

const (
	TargetPump   TargetKind = "pump"
	TargetPond   TargetKind = "pond"
	TargetSensor TargetKind = "sensor"
)

// MaintenanceTask 维护任务。
type MaintenanceTask struct {
	ID         int64
	TargetType TargetKind
	TargetID   int64
	Title      string
	Status     MaintenanceStatus
	DueAt      time.Time
	DoneAt     *time.Time
	Note       string
	CreatedAt  int64
}

// Validate 校验。
func (m *MaintenanceTask) Validate() error {
	if m.Title == "" {
		return fmt.Errorf("%w: title required", ErrInvalidInput)
	}
	switch m.TargetType {
	case TargetPump, TargetPond, TargetSensor:
	default:
		return fmt.Errorf("%w: unknown target type %q", ErrInvalidInput, m.TargetType)
	}
	if m.TargetID <= 0 {
		return fmt.Errorf("%w: target id required", ErrInvalidInput)
	}
	if m.DueAt.IsZero() {
		return fmt.Errorf("%w: due date required", ErrInvalidInput)
	}
	return nil
}

// Start 开始维护（planned → in_progress）。
func (m *MaintenanceTask) Start() error {
	if m.Status != MaintPlanned {
		return fmt.Errorf("%w: task %d in %s cannot start", ErrInvalidState, m.ID, m.Status)
	}
	m.Status = MaintInProgress
	return nil
}

// Complete 完成（in_progress → done）。
func (m *MaintenanceTask) Complete(now time.Time) error {
	if m.Status != MaintInProgress {
		return fmt.Errorf("%w: task %d in %s cannot complete", ErrInvalidState, m.ID, m.Status)
	}
	t := now
	m.Status = MaintDone
	m.DoneAt = &t
	return nil
}

// Block 标记受阻。
func (m *MaintenanceTask) Block(reason string) error {
	if m.Status == MaintDone {
		return fmt.Errorf("%w: task %d already done", ErrInvalidState, m.ID)
	}
	m.Status = MaintBlocked
	m.Note = reason
	return nil
}

// Overdue 判断是否逾期（done/blocked 不算）。
func (m *MaintenanceTask) Overdue(now time.Time) bool {
	if m.Status == MaintDone || m.Status == MaintBlocked {
		return false
	}
	return now.After(m.DueAt)
}

// EventLog 状态变更审计事件。
type EventLog struct {
	ID         int64
	Actor      string
	EntityType string
	EntityID   int64
	Action     string
	Detail     string
	OccurredAt time.Time
}

// NewEvent 构造审计事件。
func NewEvent(actor, entityType string, entityID int64, action, detail string, now time.Time) EventLog {
	return EventLog{
		Actor: actor, EntityType: entityType, EntityID: entityID,
		Action: action, Detail: detail, OccurredAt: now,
	}
}
