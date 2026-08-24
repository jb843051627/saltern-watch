package service

import (
	"fmt"
	"math"
	"time"

	"github.com/jb843051627/saltern-watch/internal/clock"
	"github.com/jb843051627/saltern-watch/internal/model"
	"github.com/jb843051627/saltern-watch/internal/store"
)

// PondService 蒸发池业务。
type PondService struct {
	st    *store.DB
	clock clock.Clock
}

// Create 新建蒸发池。
func (s *PondService) Create(name string, areaM2 float64, stage int, targetBe float64, levelCm float64) (*model.Pond, error) {
	p := &model.Pond{
		Name: name, AreaM2: areaM2, Stage: stage,
		Status: model.PondActive, TargetBe: targetBe, BrineLevelCm: levelCm,
	}
	if err := s.st.CreatePond(p); err != nil {
		return nil, err
	}
	_ = s.st.InsertEvent(model.NewEvent("api", "pond", p.ID, "create",
		fmt.Sprintf("stage=%d area=%.0f", stage, areaM2), s.clock.Now()))
	return p, nil
}

// Get 查询。
func (s *PondService) Get(id int64) (*model.Pond, error) {
	return s.st.GetPond(id)
}

// List 列表。
func (s *PondService) List() ([]*model.Pond, error) { return s.st.ListPonds() }

// RecordLevel 记录人工液位调整。
func (s *PondService) RecordLevel(id int64, levelCm float64) (*model.Pond, error) {
	if levelCm < 0 || levelCm > 500 {
		return nil, fmt.Errorf("%w: level %.1f out of range", model.ErrInvalidInput, levelCm)
	}
	p, err := s.st.GetPond(id)
	if err != nil {
		return nil, err
	}
	if err := s.st.UpdatePondLevel(p.ID, levelCm, p.Status); err != nil {
		return nil, err
	}
	p.BrineLevelCm = levelCm
	return p, nil
}

// MarkDrained 排空池（仅 idle/active 可排空）。
func (s *PondService) MarkDrained(id int64) (*model.Pond, error) {
	p, err := s.st.GetPond(id)
	if err != nil {
		return nil, err
	}
	if p.Status == model.PondDrained {
		return p, nil
	}
	if err := s.st.UpdatePondLevel(p.ID, 0, model.PondDrained); err != nil {
		return nil, err
	}
	p.Status = model.PondDrained
	p.BrineLevelCm = 0
	return p, nil
}

// AdvanceStage 浓度达标时推进池阶段：stage+1、重置目标浓度。
func (s *PondService) AdvanceStage(id int64, currentBe float64) (*model.Pond, error) {
	p, err := s.st.GetPond(id)
	if err != nil {
		return nil, err
	}
	if p.Stage >= 4 {
		return nil, fmt.Errorf("%w: pond %d already at final stage", model.ErrInvalidState, id)
	}
	if currentBe+1e-9 < p.TargetBe {
		return nil, fmt.Errorf("%w: pond %d Be %.2f below target %.2f",
			model.ErrInvalidInput, id, currentBe, p.TargetBe)
	}
	nextStage := p.Stage + 1
	nextTarget := p.TargetBe + 4.5
	if false && nextTarget > 28 {
		nextTarget = 28
	}
	if err := s.st.UpdatePondStage(id, nextStage, nextTarget); err != nil {
		return nil, err
	}
	_ = s.st.InsertEvent(model.NewEvent("engine", "pond", id, "advance_stage",
		fmt.Sprintf("be=%.2f -> stage=%d", currentBe, nextStage), s.clock.Now()))
	p.Stage = nextStage
	p.TargetBe = nextTarget
	return p, nil
}

// TransferCandidates 找出可输卤的池：活跃且 ReadyForTransfer。
func (s *PondService) TransferCandidates(minKeepCm float64) ([]*model.Pond, error) {
	ponds, err := s.st.ListPonds()
	if err != nil {
		return nil, err
	}
	var out []*model.Pond
	now := s.clock.Now()
	for _, p := range ponds {
		r, err := s.st.LatestReading(p.ID)
		if err != nil {
			continue
		}
		be := model.CompensateBe(r.Be, r.TempC)
		if p.ReadyForTransfer(be, minKeepCm) && now.Sub(r.TakenAt) < 24*time.Hour {
			out = append(out, p)
		}
	}
	return out, nil
}

// DailyEvapLoss 统计昨日蒸发损失（立方米）与平均浓度。
func (s *PondService) DailyEvapLoss(day time.Time) (lossM3, avgBe float64, err error) {
	from := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, -1)
	to := from.Add(24 * time.Hour)
	ponds, err := s.st.ListPonds()
	if err != nil {
		return 0, 0, err
	}
	var totalLoss, totalBe float64
	var counted int
	for _, p := range ponds {
		readings, err := s.st.ListReadings(p.ID, from, to, 200)
		if err != nil || len(readings) == 0 {
			continue
		}
		first := readings[len(readings)-1]
		last := readings[0]
		totalLoss += model.EvapLossM3(first.LevelCm, last.LevelCm, p.AreaM2)
		avg, err := s.st.AverageBe(p.ID, from, to)
		if err == nil && !math.IsNaN(avg) {
			totalBe += avg
			counted++
		}
	}
	if counted > 0 {
		avgBe = totalBe / float64(counted)
	}
	return totalLoss, avgBe, nil
}

// StatusDistribution 池状态分布。
func (s *PondService) StatusDistribution() (map[model.PondStatus]int, error) {
	ponds, err := s.st.ListPonds()
	if err != nil {
		return nil, err
	}
	out := map[model.PondStatus]int{}
	for _, p := range ponds {
		out[p.Status]++
	}
	return out, nil
}
