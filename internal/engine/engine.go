// Package engine 运行后台采集/评估/调度循环。
package engine

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jb843051627/saltern-watch/internal/clock"
	"github.com/jb843051627/saltern-watch/internal/config"
	"github.com/jb843051627/saltern-watch/internal/model"
	"github.com/jb843051627/saltern-watch/internal/service"
)

// Engine 后台引擎：聚合三个循环与输卤 worker 池。
type Engine struct {
	svc   *service.Service
	cfg   *config.Config
	clock clock.Clock

	jobs    chan int64 // 待执行输卤任务 ID
	stopped chan struct{}
}

// New 构造引擎。
func New(svc *service.Service, cfg *config.Config, ck clock.Clock) *Engine {
	return &Engine{
		svc: svc, cfg: cfg, clock: ck,
		jobs:    make(chan int64, 64),
		stopped: make(chan struct{}),
	}
}

// Run 启动全部后台循环，ctx 取消后优雅退出。
func (e *Engine) Run(ctx context.Context) error {
	errs := make(chan error, 4)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		e.loop(ctx, e.cfg.PollInterval, "poller", e.pollOnce)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		e.loop(ctx, e.cfg.EvalInterval, "evaluator", e.evaluateOnce)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		e.loop(ctx, e.cfg.TickInterval, "scheduler", e.scheduleOnce)
	}()

	for i := 0; i < e.cfg.WorkerCount; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			e.worker(ctx, n)
		}(i)
	}

	go func() {
		wg.Wait()
		close(e.stopped)
	}()

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		<-e.stopped
		return nil
	}
}

// loop 以固定间隔执行 fn，直到 ctx 取消。
func (e *Engine) loop(ctx context.Context, interval time.Duration, name string, fn func(context.Context) error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := fn(ctx); err != nil {
				log.Printf("engine: %s: %v", name, err)
			}
		}
	}
}

// pollOnce 轮询周期：推进熟化结晶池。
func (e *Engine) pollOnce(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	n, err := e.svc.Crystallizers.PromoteRipened()
	if err != nil {
		return fmt.Errorf("promote ripened: %w", err)
	}
	if n > 0 {
		log.Printf("engine: promoted %d crystallizers to harvest-ready", n)
	}
	return nil
}

// evaluateOnce 告警评估周期。
func (e *Engine) evaluateOnce(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if _, err := e.svc.Alerts.EvaluatePonds(); err != nil {
		return fmt.Errorf("evaluate ponds: %w", err)
	}
	if _, err := e.svc.Alerts.EvaluatePumps(); err != nil {
		return fmt.Errorf("evaluate pumps: %w", err)
	}
	if _, err := e.svc.Maintenance.ScanOverdue(); err != nil {
		return fmt.Errorf("scan overdue: %w", err)
	}
	risk, err := e.svc.Weather.RainRisk()
	if err != nil {
		return fmt.Errorf("rain risk: %w", err)
	}
	if risk == model.RainRiskAlert {
		paused, err := e.svc.Transfers.PauseAllOnRain(risk)
		if err != nil {
			return fmt.Errorf("pause on rain: %w", err)
		}
		if paused > 0 {
			log.Printf("engine: paused %d transfers by rain alert", paused)
		}
	}
	return nil
}

// scheduleOnce 调度周期：把到期任务投递给 worker。
func (e *Engine) scheduleOnce(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	jobs, err := e.svc.Store().PendingTransfers(e.clock.Now())
	if err != nil {
		return fmt.Errorf("pending transfers: %w", err)
	}
	for _, j := range jobs {
		select {
		case e.jobs <- j.ID:
		default:
			return fmt.Errorf("job queue full, deferring job %d", j.ID)
		}
	}
	return nil
}

// worker 执行输卤任务。
func (e *Engine) worker(ctx context.Context, n int) {
	for {
		select {
		case <-ctx.Done():
			return
		case id, ok := <-e.jobs:
			if !ok {
				return
			}
			job, err := e.svc.Transfers.Execute(ctx, id)
			if err != nil {
				log.Printf("engine: worker-%d job %d failed: %v", n, id, err)
				continue
			}
			log.Printf("engine: worker-%d job %d done (%.0f m3)", n, job.ID, job.VolumeM3)
		}
	}
}
