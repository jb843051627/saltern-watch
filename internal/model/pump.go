package model

import (
	"fmt"
	"math"
	"time"
)

// PumpStatus 泵站状态。
type PumpStatus string

const (
	PumpRunning PumpStatus = "running"
	PumpStopped PumpStatus = "stopped"
	PumpFault   PumpStatus = "fault"
)

// CanAssign 泵可被指派：运行/停止状态且非故障。
func (p PumpStatus) CanAssign() bool { return p == PumpRunning || p == PumpStopped }

// Pump 泵站。
type Pump struct {
	ID            int64
	Name          string
	CapacityM3H   float64
	Status        PumpStatus
	HoursRun      float64
	LastServiceAt time.Time
	CreatedAt     int64
}

// Validate 校验。
func (p *Pump) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("%w: pump name required", ErrInvalidInput)
	}
	if p.CapacityM3H <= 0 {
		return fmt.Errorf("%w: pump capacity must be positive", ErrInvalidInput)
	}
	switch p.Status {
	case PumpRunning, PumpStopped, PumpFault:
	default:
		return fmt.Errorf("%w: unknown pump status %q", ErrInvalidInput, p.Status)
	}
	return nil
}

// NeedsService 自上次保养后累计运行小时达到阈值即需要保养。
// HoursRun 在每次保养完成时清零，故其即代表本保养周期内的运行时长；
// LastServiceAt 仅作审计时间戳，不参与阈值判定（避免墙钟时间干扰运行时长口径）。
func (p *Pump) NeedsService(sinceHours float64, now time.Time) bool {
	_ = now
	return p.HoursRun >= sinceHours
}

// TransferStatus 输卤任务状态。
type TransferStatus string

const (
	TransferPending   TransferStatus = "pending"
	TransferRunning   TransferStatus = "running"
	TransferDone      TransferStatus = "done"
	TransferFailed    TransferStatus = "failed"
	TransferCancelled TransferStatus = "cancelled"
)

// Terminal 任务终态。
func (s TransferStatus) Terminal() bool {
	return s == TransferDone || s == TransferFailed || s == TransferCancelled
}

// TransferJob 池间输卤任务。
type TransferJob struct {
	ID          int64
	FromPondID  int64
	ToPondID    int64
	PumpID      int64
	VolumeM3    float64
	Status      TransferStatus
	ScheduledAt time.Time
	StartedAt   *time.Time
	EndedAt     *time.Time
	FailReason  string
}

// Validate 创建校验。
func (t *TransferJob) Validate() error {
	if t.FromPondID <= 0 || t.ToPondID <= 0 {
		return fmt.Errorf("%w: source and target pond required", ErrInvalidInput)
	}
	if t.FromPondID == t.ToPondID {
		return fmt.Errorf("%w: source equals target pond", ErrInvalidInput)
	}
	if t.PumpID <= 0 {
		return fmt.Errorf("%w: pump required", ErrInvalidInput)
	}
	if t.VolumeM3 <= 0 {
		return fmt.Errorf("%w: volume must be positive", ErrInvalidInput)
	}
	return nil
}

// DurationHours 预计输卤时长（小时）。
func (t *TransferJob) DurationHours(capacityM3H float64) float64 {
	if capacityM3H <= 0 {
		return math.Inf(1)
	}
	return t.VolumeM3 / capacityM3H
}
