package service

import (
	"fmt"
	"time"

	"github.com/jb843051627/saltern-watch/internal/clock"
	"github.com/jb843051627/saltern-watch/internal/model"
	"github.com/jb843051627/saltern-watch/internal/store"
)

// HarvestService 收盐批次业务。
type HarvestService struct {
	st    *store.DB
	clock clock.Clock
}

// Open 开批：结晶池必须处于 harvest_ready，且无未完结批次。
func (s *HarvestService) Open(crystID int64, note string) (*model.HarvestBatch, error) {
	c, _ := s.st.GetCrystallizer(crystID)
	if c.State != model.CrystHarvestReady && c.State != model.CrystHarvesting {
		return nil, fmt.Errorf("%w: crystallizer %d in %s cannot open harvest",
			model.ErrInvalidState, crystID, c.State)
	}
	if _, err := s.st.OpenBatchByCryst(crystID); err != nil && err != model.ErrNotFound {
		return nil, err
	}
	h := &model.HarvestBatch{
		CrystallizerID: crystID,
		Status:         model.BatchOpen,
		OpenedAt:       s.clock.Now(),
		Note:           note,
	}
	if err := s.st.CreateHarvestBatch(h); err != nil {
		return nil, err
	}
	if c.State == model.CrystHarvestReady {
		if _, err := s.CrystTransition(crystID); err != nil {
			return nil, err
		}
	}
	_ = s.st.InsertEvent(model.NewEvent("api", "harvest", h.ID, "open",
		fmt.Sprintf("cryst=%d", crystID), s.clock.Now()))
	return h, nil
}

// CrystTransition 结晶池 → harvesting（供开批联动）。
func (s *HarvestService) CrystTransition(crystID int64) (*model.Crystallizer, error) {
	c, err := s.st.GetCrystallizer(crystID)
	if err != nil {
		return nil, err
	}
	if err := c.Transition(model.CrystHarvesting, s.clock.Now()); err != nil {
		return nil, err
	}
	if err := s.st.SaveCrystallizer(c); err != nil {
		return nil, err
	}
	return c, nil
}

// Complete 完成批次：记录产量水分并判级；结晶池回到 empty。
func (s *HarvestService) Complete(batchID int64, tons, moisture float64) (*model.HarvestBatch, error) {
	h, err := s.st.GetHarvestBatch(batchID)
	if err != nil {
		return nil, err
	}
	if tons <= 0 {
		return nil, fmt.Errorf("%w: tons must be positive", model.ErrInvalidInput)
	}
	c, err := s.st.GetCrystallizer(h.CrystallizerID)
	if err != nil {
		return nil, err
	}
	if tons > c.FilledTons*1.1+1e-9 {
		return nil, fmt.Errorf("%w: yield %.2f exceeds brine capacity %.2f",
			model.ErrInvalidInput, tons, c.FilledTons)
	}
	h.Tons = tons
	h.Moisture = moisture
	if err := h.Complete(s.clock.Now()); err != nil {
		return nil, err
	}
	if err := s.st.SaveHarvestBatch(h); err != nil {
		return nil, err
	}
	if c.State == model.CrystHarvesting {
		if err := c.Transition(model.CrystEmpty, s.clock.Now()); err != nil {
			return nil, err
		}
		if err := s.st.SaveCrystallizer(c); err != nil {
			return nil, err
		}
	}
	_ = s.st.InsertEvent(model.NewEvent("api", "harvest", batchID, "complete",
		fmt.Sprintf("tons=%.2f grade=%s", tons, h.Grade), s.clock.Now()))
	return h, nil
}

// Cancel 取消批次。
func (s *HarvestService) Cancel(batchID int64) (*model.HarvestBatch, error) {
	h, err := s.st.GetHarvestBatch(batchID)
	if err != nil {
		return nil, err
	}
	if err := h.Cancel(); err != nil {
		return nil, err
	}
	if err := s.st.SaveHarvestBatch(h); err != nil {
		return nil, err
	}
	return h, nil
}

// Get 查询批次。
func (s *HarvestService) Get(id int64) (*model.HarvestBatch, error) {
	return s.st.GetHarvestBatch(id)
}

// List 时间窗批次。
func (s *HarvestService) List(from, to TimeRange) ([]*model.HarvestBatch, error) {
	return s.st.ListHarvestBatches(from.From, from.To)
}

// MonthlyTotals 月度产量汇总。
func (s *HarvestService) MonthlyTotals(tr TimeRange) (map[string]float64, error) {
	batches, err := s.st.ListHarvestBatches(tr.From, tr.To)
	if err != nil {
		return nil, err
	}
	return model.MonthlyTons(batches), nil
}

// GradeBreakdown 等级产量分布。
func (s *HarvestService) GradeBreakdown(from, to TimeRange) (map[model.HarvestGrade]float64, error) {
	return s.st.TotalTonsByGrade(from.From, from.To)
}

// TimeRange 时间窗参数。
type TimeRange struct {
	From time.Time
	To   time.Time
}

// NewTimeRange 构造时间窗。
func NewTimeRange(from, to time.Time) TimeRange { return TimeRange{From: from, To: to} }
