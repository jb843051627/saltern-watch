// Package service 实现盐田监控业务逻辑。
package service

import (
	"github.com/jb843051627/saltern-watch/internal/clock"
	"github.com/jb843051627/saltern-watch/internal/config"
	"github.com/jb843051627/saltern-watch/internal/store"
)

// Service 聚合各业务子服务。
type Service struct {
	Ponds         *PondService
	Brine         *BrineService
	Crystallizers *CrystallizerService
	Harvests      *HarvestService
	Pumps         *PumpService
	Transfers     *TransferService
	Weather       *WeatherService
	Alerts        *AlertService
	Maintenance   *MaintenanceService
	Reports       *ReportService
	Quality       *QualityService
	Sensors       *SensorService
	PondGroups    *PondGroupService

	store *store.DB
	clock clock.Clock
	cfg   *config.Config
}

// New 构造聚合服务。
func New(st *store.DB, ck clock.Clock, cfg *config.Config) *Service {
	s := &Service{store: st, clock: ck, cfg: cfg}
	s.Ponds = &PondService{st: st, clock: ck}
	s.Brine = &BrineService{st: st, clock: ck, cfg: cfg}
	s.Crystallizers = &CrystallizerService{st: st, clock: ck}
	s.Harvests = &HarvestService{st: st, clock: ck}
	s.Pumps = &PumpService{st: st, clock: ck}
	s.Transfers = &TransferService{st: st, clock: ck}
	s.Weather = &WeatherService{st: st, clock: ck}
	s.Alerts = &AlertService{st: st, clock: ck}
	s.Maintenance = &MaintenanceService{st: st, clock: ck}
	s.Reports = &ReportService{st: st, clock: ck, zone: cfg.LocalZone}
	s.Quality = &QualityService{st: st, clock: ck}
	s.Sensors = &SensorService{st: st, clock: ck}
	s.PondGroups = &PondGroupService{st: st, clock: ck}
	s.Brine.sensors = s.Sensors
	return s
}

// Store 暴露存储层给 engine 使用。
func (s *Service) Store() *store.DB { return s.store }
