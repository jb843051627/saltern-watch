// Package clock 提供可注入的时间源，便于调度与统计逻辑测试。
package clock

import (
	"sync"
	"time"
)

// Clock 业务代码统一通过该接口取时间。
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// Real 返回真实时间源。
func Real() Clock { return realClock{} }

// Fake 可手动推进的假时钟（仅测试使用）。
type Fake struct {
	mu  sync.Mutex
	now time.Time
}

// NewFake 构造从指定时刻开始的假时钟。
func NewFake(start time.Time) *Fake { return &Fake{now: start} }

// Now 返回当前假时间。
func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Advance 向前推进指定时长。
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	f.now = f.now.Add(d)
	f.mu.Unlock()
}
