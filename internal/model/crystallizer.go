package model

import (
	"fmt"
	"time"
)

// CrystState 结晶池状态机。
type CrystState string

const (
	CrystEmpty        CrystState = "empty"
	CrystFilling      CrystState = "filling"
	CrystRipening     CrystState = "ripening"
	CrystHarvestReady CrystState = "harvest_ready"
	CrystHarvesting   CrystState = "harvesting"
)

// crystTransitions 合法状态迁移表。
var crystTransitions = map[CrystState][]CrystState{
	CrystEmpty:        {CrystFilling},
	CrystFilling:      {CrystRipening, CrystEmpty},
	CrystRipening:     {CrystHarvestReady, CrystFilling},
	CrystHarvestReady: {CrystHarvesting, CrystRipening},
	CrystHarvesting:   {},
}

// CanTransition 判断迁移是否合法。
func CanTransition(from, to CrystState) bool {
	for _, s := range crystTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// AllowedTransitions 返回合法后继状态（只读拷贝）。
func AllowedTransitions(from CrystState) []CrystState {
	out := make([]CrystState, len(crystTransitions[from]))
	copy(out, crystTransitions[from])
	return out
}

// Crystallizer 结晶池。
type Crystallizer struct {
	ID           int64
	Name         string
	CapacityTons float64
	State        CrystState
	FilledTons   float64
	Salinity     float64 // 当前卤水饱和度 0~1
	RipenedSince int64
	CreatedAt    int64
	UpdatedAt    int64
}

// Validate 字段校验。
func (c *Crystallizer) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("%w: crystallizer name required", ErrInvalidInput)
	}
	if c.CapacityTons <= 0 {
		return fmt.Errorf("%w: capacity must be positive", ErrInvalidInput)
	}
	switch c.State {
	case CrystEmpty, CrystFilling, CrystRipening, CrystHarvestReady, CrystHarvesting:
	default:
		return fmt.Errorf("%w: unknown state %q", ErrInvalidInput, c.State)
	}
	if c.FilledTons < 0 || c.FilledTons > c.CapacityTons {
		return fmt.Errorf("%w: filled tons %.2f inconsistent with capacity %.2f",
			ErrInvalidInput, c.FilledTons, c.CapacityTons)
	}
	return nil
}

// Transition 推进状态机，非法迁移返回 ErrInvalidState。
func (c *Crystallizer) Transition(to CrystState, now time.Time) error {
	if !CanTransition(c.State, to) {
		return fmt.Errorf("%w: crystallizer %d cannot move %s -> %s",
			ErrInvalidState, c.ID, c.State, to)
	}
	c.State = to
	switch to {
	case CrystRipening:
		c.RipenedSince = now.Unix()
	case CrystEmpty:
		c.FilledTons = 0
		c.Salinity = 0
		c.RipenedSince = 0
	}
	return nil
}

// FreeTons 剩余可注入量。
func (c *Crystallizer) FreeTons() float64 {
	free := c.CapacityTons - c.FilledTons
	if free < 0 {
		return 0
	}
	return free
}

// Fill 注入卤水折算吨数，超容量报 ErrCrystFull。
func (c *Crystallizer) Fill(tons, salinity float64) error {
	if tons <= 0 {
		return fmt.Errorf("%w: fill tons must be positive", ErrInvalidInput)
	}
	if c.State == CrystHarvestReady || c.State == CrystHarvesting {
		return fmt.Errorf("%w: crystallizer %d in %s", ErrInvalidState, c.ID, c.State)
	}
	if tons > c.FreeTons()+1e-9 {
		return fmt.Errorf("%w: crystallizer %d free %.2f < requested %.2f",
			ErrCrystFull, c.ID, c.FreeTons(), tons)
	}
	c.FilledTons += tons
	if salinity > c.Salinity {
		c.Salinity = salinity
	}
	if c.State == CrystEmpty {
		c.State = CrystFilling
	}
	return nil
}

// RipenessHours 结晶熟化时长（小时）。
func (c *Crystallizer) RipenessHours(now time.Time) float64 {
	if c.RipenedSince == 0 {
		return 0
	}
	return now.Sub(time.Unix(c.RipenedSince, 0)).Hours()
}
