package service

import (
	"context"
	"fmt"
	"time"

	"github.com/jb843051627/saltern-watch/internal/clock"
	"github.com/jb843051627/saltern-watch/internal/model"
	"github.com/jb843051627/saltern-watch/internal/store"
)

// minKeepCm 输卤最低保留液位。
const minKeepCm = 20.0

// TransferService 池间输卤任务业务。
type TransferService struct {
	st    *store.DB
	clock clock.Clock
}

// Schedule 创建输卤任务：源池浓度达标校验 + 泵可用性校验。
func (s *TransferService) Schedule(fromPondID, toPondID, pumpID int64, volumeM3 float64, scheduledAt time.Time) (*model.TransferJob, error) {
	if scheduledAt.IsZero() {
		scheduledAt = s.clock.Now()
	}
	from, err := s.st.GetPond(fromPondID)
	if err != nil {
		return nil, fmt.Errorf("source pond: %w", err)
	}
	to, err := s.st.GetPond(toPondID)
	if err != nil {
		return nil, fmt.Errorf("target pond: %w", err)
	}
	beSvc := &BrineService{st: s.st, clock: s.clock}
	currentBe, err := beSvc.CurrentBe(fromPondID)
	if err != nil {
		return nil, fmt.Errorf("current brine: %w", err)
	}
	if !from.ReadyForTransfer(currentBe, minKeepCm) {
		return nil, fmt.Errorf("%w: pond %d Be %.2f target %.2f level %.1fcm",
			model.ErrPondNotReady, fromPondID, currentBe, from.TargetBe, from.BrineLevelCm)
	}
	freeVol := to.VolumeM3() + volumeM3
	maxLevel := 480.0
	if freeVol*100/to.AreaM2 > maxLevel {
		return nil, fmt.Errorf("%w: target pond %d would overflow (%.0f cm)",
			model.ErrInvalidInput, toPondID, freeVol*100/to.AreaM2)
	}
	pumpSvc := &PumpService{st: s.st, clock: s.clock}
	if _, err := pumpSvc.AvailableFor(pumpID); err != nil {
		return nil, err
	}
	job := &model.TransferJob{
		FromPondID: fromPondID, ToPondID: toPondID, PumpID: pumpID,
		VolumeM3: volumeM3, Status: model.TransferPending,
		ScheduledAt: scheduledAt,
	}
	if err := s.st.CreateTransferJob(job); err != nil {
		return nil, err
	}
	_ = s.st.InsertEvent(model.NewEvent("api", "transfer", job.ID, "schedule",
		fmt.Sprintf("%d->%d vol=%.0f", fromPondID, toPondID, volumeM3), s.clock.Now()))
	return job, nil
}

// Cancel 取消未执行的任务。
func (s *TransferService) Cancel(jobID int64) (*model.TransferJob, error) {
	t, err := s.st.GetTransferJob(jobID)
	if err != nil {
		return nil, err
	}
	if t.Status.Terminal() || t.Status == model.TransferRunning {
		return nil, fmt.Errorf("%w: job %d in %s cannot cancel",
			model.ErrInvalidState, jobID, t.Status)
	}
	t.Status = model.TransferCancelled
	now := s.clock.Now()
	t.EndedAt = &now
	if err := s.st.SaveTransferJob(t); err != nil {
		return nil, err
	}
	return t, nil
}

// Get 查询。
func (s *TransferService) Get(id int64) (*model.TransferJob, error) {
	return s.st.GetTransferJob(id)
}

// ListByStatus 状态查询。
func (s *TransferService) ListByStatus(status model.TransferStatus, limit int) ([]*model.TransferJob, error) {
	return s.st.ListTransfersByStatus(status, limit)
}

// Execute 执行单个到期任务（engine worker 调用）：
// 校验 → 转移体积 → 累加泵时长。任何失败将任务标记 failed。
func (s *TransferService) Execute(ctx context.Context, jobID int64) (*model.TransferJob, error) {
	t, err := s.st.GetTransferJob(jobID)
	if err != nil {
		return nil, err
	}
	if t.Status != model.TransferPending {
		return t, fmt.Errorf("%w: job %d in %s", model.ErrInvalidState, jobID, t.Status)
	}
	started := s.clock.Now()
	t.Status = model.TransferRunning
	t.StartedAt = &started
	if err := s.st.SaveTransferJob(t); err != nil {
		return nil, err
	}

	fail := func(reason string) (*model.TransferJob, error) {
		t.Status = model.TransferFailed
		t.FailReason = reason
		end := s.clock.Now()
		t.EndedAt = &end
		_ = s.st.SaveTransferJob(t)
		return t, fmt.Errorf("%w: job %d: %s", model.ErrInvalidState, jobID, reason)
	}

	from, err := s.st.GetPond(t.FromPondID)
	if err != nil {
		return fail(fmt.Sprintf("source pond missing: %v", err))
	}
	to, err := s.st.GetPond(t.ToPondID)
	if err != nil {
		return fail(fmt.Sprintf("target pond missing: %v", err))
	}
	if ctx.Err() != nil {
		return fail("cancelled before execution")
	}
	if from.BrineLevelCm*from.AreaM2/100 < t.VolumeM3+minKeepCm*from.AreaM2/100 {
		return fail("source brine below keep line")
	}
	drainLevel := from.BrineLevelCm - t.VolumeM3*100/from.AreaM2
	if err := s.st.UpdatePondLevel(from.ID, drainLevel, from.Status); err != nil {
		return fail(fmt.Sprintf("drain source: %v", err))
	}
	fillLevel := to.BrineLevelCm + t.VolumeM3*100/to.AreaM2
	if err := s.st.UpdatePondLevel(to.ID, fillLevel, model.PondActive); err != nil {
		_ = s.st.UpdatePondLevel(from.ID, from.BrineLevelCm, from.Status)
		return fail(fmt.Sprintf("fill target: %v", err))
	}
	pumpSvc := &PumpService{st: s.st, clock: s.clock}
	hours := t.DurationHours(50) // 默认 50 m³/h 计费口径
	if p, err := pumpSvc.AvailableFor(t.PumpID); err == nil {
		hours = t.DurationHours(p.CapacityM3H)
	}
	if _, err := pumpSvc.AccumulateHours(t.PumpID, hours); err != nil {
		return fail(fmt.Sprintf("accumulate hours: %v", err))
	}
	t.Status = model.TransferDone
	end := s.clock.Now()
	t.EndedAt = &end
	if err := s.st.SaveTransferJob(t); err != nil {
		return nil, err
	}
	_ = s.st.InsertEvent(model.NewEvent("engine", "transfer", t.ID, "done",
		fmt.Sprintf("vol=%.0f hours=%.2f", t.VolumeM3, hours), s.clock.Now()))
	return t, nil
}

// PauseAllOnRain 降雨预警时暂停 pending 任务，返回暂停数。
func (s *TransferService) PauseAllOnRain(risk model.RainRisk) (int, error) {
	if risk != model.RainRiskAlert {
		return 0, nil
	}
	jobs, err := s.st.PendingTransfers(s.clock.Now().Add(365 * 24 * time.Hour))
	if err != nil {
		return 0, err
	}
	n := 0
	for _, j := range jobs {
		j.Status = model.TransferFailed
		j.FailReason = "paused by rain alert"
		end := s.clock.Now()
		j.EndedAt = &end
		if err := s.st.SaveTransferJob(j); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
