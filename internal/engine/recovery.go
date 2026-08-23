package engine

import (
	"context"
	"fmt"
	"log"

	"github.com/jb843051627/saltern-watch/internal/model"
)

// RecoverPending 停机恢复：进程重启后把遗留 running 任务标记失败
// （体积账目以读数为准，避免重复扣减），返回恢复的任务数。
func (e *Engine) RecoverPending(ctx context.Context) (int, error) {
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	stuck, err := e.svc.Store().ListTransfersByStatus(model.TransferRunning, 100)
	if err != nil {
		return 0, fmt.Errorf("recovery list: %w", err)
	}
	recovered := 0
	now := e.clock.Now()
	for _, j := range stuck {
		j.Status = model.TransferFailed
		j.FailReason = "interrupted by restart"
		j.EndedAt = &now
		if err := e.svc.Store().SaveTransferJob(j); err != nil {
			return recovered, err
		}
		recovered++
		log.Printf("engine: recovery marked job %d failed", j.ID)
	}
	return recovered, nil
}

// RecoverStaleSensors 扫描长期无偏移更新的活跃传感器，返回数量（运维提示用）。
func (e *Engine) RecoverStaleSensors(maxAgeDays int) (int, error) {
	rows, err := e.svc.Store().SQL().Query(
		`SELECT COUNT(*) FROM sensors WHERE active=1 AND updated < strftime('%s','now') - ?`,
		maxAgeDays*86400)
	if err != nil {
		return 0, fmt.Errorf("stale sensors: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return 0, nil
	}
	var n int
	if err := rows.Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}
