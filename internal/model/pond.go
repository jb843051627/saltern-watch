// Package model 定义盐田监控领域实体。
package model

import (
	"errors"
	"fmt"
)

// 领域哨兵错误：上层用 errors.Is 判定语义。
var (
	ErrNotFound        = errors.New("saltern: entity not found")
	ErrDuplicate       = errors.New("saltern: duplicate entity")
	ErrInvalidInput    = errors.New("saltern: invalid input")
	ErrInvalidState    = errors.New("saltern: invalid state transition")
	ErrConflict        = errors.New("saltern: concurrent modification")
	ErrPondNotReady    = errors.New("saltern: pond not ready for transfer")
	ErrCrystFull       = errors.New("saltern: crystallizer at capacity")
	ErrHarvestOpen     = errors.New("saltern: open harvest batch exists")
	ErrPumpUnavailable = errors.New("saltern: pump unavailable")
)

// PondStatus 蒸发池状态。
type PondStatus string

const (
	PondActive  PondStatus = "active"
	PondIdle    PondStatus = "idle"
	PondDrained PondStatus = "drained"
)

// Pond 蒸发池。海水沿 stage 0→4 逐级浓缩。
type Pond struct {
	ID           int64
	Name         string
	AreaM2       float64
	Stage        int
	Status       PondStatus
	BrineLevelCm float64 // 当前液位
	TargetBe     float64 // 进入下一阶段的目标浓度（Bé）
	CreatedAt    int64
	UpdatedAt    int64
}

// Validate 创建/更新时的字段校验。
func (p *Pond) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("%w: pond name required", ErrInvalidInput)
	}
	if p.AreaM2 <= 0 {
		return fmt.Errorf("%w: pond area must be positive", ErrInvalidInput)
	}
	if p.Stage < 0 || p.Stage > 4 {
		return fmt.Errorf("%w: pond stage must be in [0,4]", ErrInvalidInput)
	}
	switch p.Status {
	case PondActive, PondIdle, PondDrained:
	default:
		return fmt.Errorf("%w: unknown pond status %q", ErrInvalidInput, p.Status)
	}
	if p.TargetBe <= 0 || p.TargetBe > 30 {
		return fmt.Errorf("%w: target Be out of range", ErrInvalidInput)
	}
	return nil
}

// VolumeM3 由液位与面积估算当前卤水体积（立方米）。
func (p *Pond) VolumeM3() float64 {
	return p.BrimeVolume(p.BrineLevelCm)
}

// BrimeVolume 计算给定液位下的体积。
func (p *Pond) BrimeVolume(levelCm float64) float64 {
	return p.AreaM2 * levelCm / 100.0
}

// ReadyForTransfer 判断是否达到输卤条件：活跃、浓度达标、液位不低于最低保留线。
func (p *Pond) ReadyForTransfer(currentBe, minKeepCm float64) bool {
	if p.Status != PondActive {
		return false
	}
	if currentBe+1e-9 < p.TargetBe {
		return false
	}
	return p.BrineLevelCm >= minKeepCm
}
