package service

import (
	"context"
	"fmt"
	"time"

	"github.com/jb843051627/saltern-watch/internal/clock"
	"github.com/jb843051627/saltern-watch/internal/config"
	"github.com/jb843051627/saltern-watch/internal/model"
	"github.com/jb843051627/saltern-watch/internal/store"
)

// BrineService 卤水读数采集与校验。
type BrineService struct {
	st    *store.DB
	clock clock.Clock
	cfg   *config.Config
}

// Ingest 单条读数采集：校验 → 温度补偿 → 突变标记 → 落库。
// 返回补偿后的浓度供调用方使用。
func (s *BrineService) Ingest(pondID int64, be, tempC, levelCm float64, source string, takenAt time.Time) (*model.BrineReading, error) {
	if _, err := s.st.GetPond(pondID); err != nil {
		return nil, err
	}
	r := &model.BrineReading{
		PondID: pondID, TakenAt: takenAt,
		Be: be, TempC: tempC, LevelCm: levelCm, Source: source,
	}
	if err := model.ValidateReading(r); err != nil {
		return nil, err
	}
	prev, err := s.st.LatestReading(pondID)
	if err == nil {
		r.Flag = model.FlagOf(prev, r)
	} else if err != model.ErrNotFound {
		return nil, err
	}
	if r.Flag == model.FlagRejected {
		if err := s.st.InsertReading(r); err != nil {
			return nil, err
		}
		return r, nil
	}
	r.Be = model.CompensateBe(r.Be, r.TempC)
	if err := s.st.InsertReading(r); err != nil {
		return nil, err
	}
	if r.Flag == model.FlagOK {
		if err := s.st.UpdatePondLevel(pondID, levelCm, model.PondActive); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// BatchIngest 批量读数：逐条处理，遇错中止；支持 ctx 取消（已处理条数保留）。
func (s *BrineService) BatchIngest(ctx context.Context, items []*model.BrineReading) (accepted int, err error) {
	if len(items) > s.cfg.MaxBatchReads {
		return 0, fmt.Errorf("%w: batch size %d exceeds limit %d",
			model.ErrInvalidInput, len(items), s.cfg.MaxBatchReads)
	}
	for i, item := range items {
		if err := ctx.Err(); err != nil {
			return accepted, fmt.Errorf("batch cancelled after %d items: %w", accepted, err)
		}
		takenAt := item.TakenAt
		if takenAt.IsZero() {
			takenAt = s.clock.Now()
		}
		if _, err := s.Ingest(item.PondID, item.Be, item.TempC, item.LevelCm, item.Source, takenAt); err != nil {
			return accepted, fmt.Errorf("item %d: %w", i, err)
		}
		accepted++
	}
	return accepted, nil
}

// Latest 查询池最新读数。
func (s *BrineService) Latest(pondID int64) (*model.BrineReading, error) {
	return s.st.LatestReading(pondID)
}

// History 历史读数。
func (s *BrineService) History(pondID int64, from, to time.Time, limit int) ([]*model.BrineReading, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	return s.st.ListReadings(pondID, from, to, limit)
}

// CurrentBe 当前补偿浓度：无有效读数返回错误。
func (s *BrineService) CurrentBe(pondID int64) (float64, error) {
	r, err := s.st.LatestReading(pondID)
	if err != nil {
		return 0, err
	}
	if r.Flag == model.FlagRejected {
		return 0, fmt.Errorf("%w: latest reading of pond %d rejected", model.ErrInvalidState, pondID)
	}
	return model.CompensateBe(r.Be, r.TempC), nil
}

// StalePonds 超过 staleDuration 无有效读数的活跃池。
func (s *BrineService) StalePonds(stale time.Duration) ([]*model.Pond, error) {
	ponds, err := s.st.ListPonds()
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	var out []*model.Pond
	for _, p := range ponds {
		if p.Status != model.PondActive {
			continue
		}
		r, err := s.st.LatestReading(p.ID)
		if err == model.ErrNotFound || (err == nil && now.Sub(r.TakenAt) > stale) {
			out = append(out, p)
		} else if err != nil && err != model.ErrNotFound {
			return nil, err
		}
	}
	return out, nil
}

// RejectedCount 窗口内被拒读数统计。
func (s *BrineService) RejectedCount(from, to time.Time) (int, error) {
	return s.st.CountRejected(from, to)
}
