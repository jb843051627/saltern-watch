package service

import (
	"sync"

	"github.com/jb843051627/saltern-watch/internal/model"
)

// PondSnapshot 单池最新读数快照。
type PondSnapshot struct {
	PondID int64
	Be     float64
	TempC  float64
	LevelCm float64
	Flag   model.ReadingFlag
}

// ReadingCache 进程内读数快照缓存（引擎评估循环与 HTTP 查询共享）。
// 并发安全：读写均持 RWMutex；对外暴露的切片必须拷贝。
type ReadingCache struct {
	mu        sync.RWMutex
	snapshots map[int64]PondSnapshot
	order     []int64 // 按 PondID 升序维护的键序
}

// NewReadingCache 构造空缓存。
func NewReadingCache() *ReadingCache {
	return &ReadingCache{snapshots: map[int64]PondSnapshot{}}
}

// Update 写入/覆盖单池快照。
func (c *ReadingCache) Update(s PondSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.snapshots[s.PondID]; !exists {
		c.order = append(c.order, s.PondID)
	}
	c.snapshots[s.PondID] = s
}

// Get 读取单池快照（值拷贝，第二返回值表示是否存在）。
func (c *ReadingCache) Get(pondID int64) (PondSnapshot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s, ok := c.snapshots[pondID]
	return s, ok
}

// All 返回全部快照的拷贝列表（按 PondID 升序），调用方修改不影响缓存。
func (c *ReadingCache) All() []PondSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]PondSnapshot, 0, len(c.order))
	for _, id := range c.order {
		out = append(out, c.snapshots[id])
	}
	return out
}

// PruneBefore 清理指定池集合之外的快照（池被排空/删除后调用）。
func (c *ReadingCache) PruneBefore(keep map[int64]bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id := range c.snapshots {
		if !keep[id] {
			delete(c.snapshots, id)
		}
	}
	next := c.order[:0]
	for _, id := range c.order {
		if _, ok := c.snapshots[id]; ok {
			next = append(next, id)
		}
	}
	c.order = next
}
