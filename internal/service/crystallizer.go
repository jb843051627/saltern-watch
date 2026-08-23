package service

import (
	"fmt"

	"github.com/jb843051627/saltern-watch/internal/clock"
	"github.com/jb843051627/saltern-watch/internal/model"
	"github.com/jb843051627/saltern-watch/internal/store"
)

// CrystallizerService 结晶池业务。
type CrystallizerService struct {
	st    *store.DB
	clock clock.Clock
}

// Create 新建结晶池。
func (s *CrystallizerService) Create(name string, capacityTons float64) (*model.Crystallizer, error) {
	c := &model.Crystallizer{Name: name, CapacityTons: capacityTons, State: model.CrystEmpty}
	if err := s.st.CreateCrystallizer(c); err != nil {
		return nil, err
	}
	_ = s.st.InsertEvent(model.NewEvent("api", "crystallizer", c.ID, "create",
		fmt.Sprintf("capacity=%.1f", capacityTons), s.clock.Now()))
	return c, nil
}

// Get 查询。
func (s *CrystallizerService) Get(id int64) (*model.Crystallizer, error) {
	return s.st.GetCrystallizer(id)
}

// List 列表。
func (s *CrystallizerService) List() ([]*model.Crystallizer, error) {
	return s.st.ListCrystallizers()
}

// FillBrine 注入饱和卤水：状态守卫 + 容量校验 + 落库。
func (s *CrystallizerService) FillBrine(id int64, tons, salinity float64) (*model.Crystallizer, error) {
	c, err := s.st.GetCrystallizer(id)
	if err != nil {
		return nil, err
	}
	if salinity < 0.85 {
		return nil, fmt.Errorf("%w: brine salinity %.2f below saturation 0.85",
			model.ErrInvalidInput, salinity)
	}
	if err := c.Fill(tons, salinity); err != nil {
		return nil, err
	}
	if err := s.st.SaveCrystallizer(c); err != nil {
		return nil, err
	}
	_ = s.st.InsertEvent(model.NewEvent("api", "crystallizer", id, "fill",
		fmt.Sprintf("tons=%.2f salinity=%.2f", tons, salinity), s.clock.Now()))
	return c, nil
}

// Transition 状态推进。
func (s *CrystallizerService) Transition(id int64, to model.CrystState) (*model.Crystallizer, error) {
	c, err := s.st.GetCrystallizer(id)
	if err != nil {
		return nil, err
	}
	if err := c.Transition(to, s.clock.Now()); err != nil {
		return nil, err
	}
	// filling → ripening 需要卤水基本注满（≥80%）。
	if to == model.CrystRipening && c.FilledTons < 0.8*c.CapacityTons {
		return nil, fmt.Errorf("%w: crystallizer %d filled %.2f/%.2f below ripening threshold",
			model.ErrInvalidState, id, c.FilledTons, c.CapacityTons)
	}
	// → harvest_ready 需要熟化 ≥72h。
	if to == model.CrystHarvestReady && c.RipenessHours(s.clock.Now()) < 72 {
		return nil, fmt.Errorf("%w: crystallizer %d ripened %.1fh < 72h",
			model.ErrInvalidState, id, c.RipenessHours(s.clock.Now()))
	}
	if err := s.st.SaveCrystallizer(c); err != nil {
		return nil, err
	}
	_ = s.st.InsertEvent(model.NewEvent("api", "crystallizer", id, "transition",
		string(to), s.clock.Now()))
	return c, nil
}

// PromoteRipened 扫描熟化期满的 ripening 池推进到 harvest_ready。
func (s *CrystallizerService) PromoteRipened() (int, error) {
	list, err := s.st.ListCrystallizers()
	if err != nil {
		return 0, err
	}
	now := s.clock.Now()
	promoted := 0
	for _, c := range list {
		if c.State != model.CrystRipening {
			continue
		}
		if c.RipenessHours(now) >= 72 && c.Salinity >= 0.95 {
			c.State = model.CrystHarvestReady
			if err := s.st.SaveCrystallizer(c); err != nil {
				return promoted, err
			}
			promoted++
		}
	}
	return promoted, nil
}

// HarvestReadyList 待收盐池。
func (s *CrystallizerService) HarvestReadyList() ([]*model.Crystallizer, error) {
	all, err := s.st.ListCrystallizers()
	if err != nil {
		return nil, err
	}
	var out []*model.Crystallizer
	for _, c := range all {
		if c.State == model.CrystHarvestReady {
			out = append(out, c)
		}
	}
	return out, nil
}

// StateDistribution 状态分布统计。
func (s *CrystallizerService) StateDistribution() (map[model.CrystState]int, error) {
	return s.st.CountCrystallizersByState()
}

// EstimateYield 预估可收盐量（按注满比例折算）。
func (s *CrystallizerService) EstimateYield(c *model.Crystallizer) float64 {
	if c.CapacityTons <= 0 {
		return 0
	}
	ratio := c.FilledTons / c.CapacityTons
	yield := c.CapacityTons * ratio * 0.32 // 析出率经验值
	now := s.clock.Now()
	if c.State == model.CrystRipening && c.RipenessHours(now) > 0 {
		maturity := c.RipenessHours(now) / 72.0
		if maturity > 1 {
			maturity = 1
		}
		yield *= maturity
	}
	return yield
}
