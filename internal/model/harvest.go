package model

import (
	"fmt"
	"sort"
	"time"
)

// HarvestGrade 收盐等级。
type HarvestGrade string

const (
	GradeSuper  HarvestGrade = "super"
	GradeFirst  HarvestGrade = "first"
	GradeSecond HarvestGrade = "second"
)

// BatchStatus 收盐批次状态。
type BatchStatus string

const (
	BatchOpen      BatchStatus = "open"
	BatchCompleted BatchStatus = "completed"
	BatchCancelled BatchStatus = "cancelled"
)

// MoistureLimit 各等级水分上限（%）。
var MoistureLimit = map[HarvestGrade]float64{
	GradeSuper:  2.0,
	GradeFirst:  3.5,
	GradeSecond: 5.0,
}

// HarvestBatch 收盐批次。
type HarvestBatch struct {
	ID             int64
	CrystallizerID int64
	Status         BatchStatus
	Tons           float64
	Moisture       float64
	Grade          HarvestGrade
	OpenedAt       time.Time
	CompletedAt    *time.Time
	Note           string
}

// Validate 开批校验。
func (h *HarvestBatch) Validate() error {
	if h.CrystallizerID <= 0 {
		return fmt.Errorf("%w: crystallizer required", ErrInvalidInput)
	}
	if h.Tons < 0 {
		return fmt.Errorf("%w: tons must be >= 0", ErrInvalidInput)
	}
	if h.Moisture < 0 || h.Moisture > 20 {
		return fmt.Errorf("%w: moisture out of range", ErrInvalidInput)
	}
	return nil
}

// GradeOf 按水分与吨位判级：水分超限降级，粒度过小（<8 吨）不评 super。
func GradeOf(moisture, tons float64) HarvestGrade {
	limit := MoistureLimit[GradeFirst]
	if tons >= 8 && moisture <= limit {
		return GradeSuper
	}
	if moisture <= MoistureLimit[GradeSuper]+2 {
		return GradeFirst
	}
	return GradeSecond
}

// Complete 完成批次：补齐完成时间并按指标判级。
func (h *HarvestBatch) Complete(now time.Time) error {
	if h.Status != BatchOpen {
		return fmt.Errorf("%w: batch %d in %s", ErrInvalidState, h.ID, h.Status)
	}
	t := now
	h.CompletedAt = &t
	h.Grade = GradeOf(h.Moisture, h.Tons)
	h.Status = BatchCompleted
	return nil
}

// Cancel 取消批次（仅 open 可取消）。
func (h *HarvestBatch) Cancel() error {
	if h.Status != BatchOpen {
		return fmt.Errorf("%w: batch %d in %s", ErrInvalidState, h.ID, h.Status)
	}
	h.Status = BatchCancelled
	return nil
}

// SortBatchesByDate 按开批时间升序排序（返回新切片，不改调用方输入顺序之外的共享数据由调用方保证）。
func SortBatchesByDate(batches []*HarvestBatch) []*HarvestBatch {
	out := make([]*HarvestBatch, len(batches))
	copy(out, batches)
	sort.Slice(out, func(i, j int) bool {
		return out[i].OpenedAt.Before(out[j].OpenedAt)
	})
	return out
}

// MonthlyTons 汇总各自然月产量（吨），键格式 2006-01。
func MonthlyTons(batches []*HarvestBatch) map[string]float64 {
	total := map[string]float64{}
	for _, b := range batches {
		if b.Status != BatchCompleted {
			continue
		}
		key := b.OpenedAt.Format("2006-01")
		total[key] += b.Tons
	}
	return total
}
