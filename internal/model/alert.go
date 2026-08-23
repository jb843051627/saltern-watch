package model

import (
	"fmt"
	"strings"
	"time"
)

// AlertSeverity 告警级别。
type AlertSeverity string

const (
	SevInfo AlertSeverity = "info"
	SevWarn AlertSeverity = "warn"
	SevCrit AlertSeverity = "crit"
)

// Escalate 告警升级：info→warn→crit，crit 保持不变。
func (s AlertSeverity) Escalate() AlertSeverity {
	switch s {
	case SevInfo:
		return SevWarn
	case SevWarn:
		return SevCrit
	default:
		return SevCrit
	}
}

// AlertStatus 告警状态。
type AlertStatus string

const (
	AlertOpen  AlertStatus = "open"
	AlertAcked AlertStatus = "acked"
	AlertClosed AlertStatus = "closed"
)

// CanAcknowledge open 状态才能确认。
func CanAcknowledge(s AlertStatus) bool { return s == AlertOpen }

// CanClose 已确认（acked）或 open 的告警可关闭；closed 不可重复关闭。
func CanClose(s AlertStatus) bool { return s == AlertOpen || s == AlertAcked }

// Alert 告警实体。DedupKey 相同的重复事件聚合计数而非新建。
type Alert struct {
	ID          int64
	DedupKey    string
	Kind        string // low_be / high_temp / level_drop / pump_fault / rain_risk / maintenance_overdue
	Severity    AlertSeverity
	Status      AlertStatus
	SubjectType string // pond / pump / crystallizer
	SubjectID   int64
	Message     string
	Count       int
	FirstSeenAt time.Time
	LastSeenAt  time.Time
	AckedAt     *time.Time
	ClosedAt    *time.Time
}

// NewAlert 构造告警并生成去重键。
func NewAlert(kind string, sev AlertSeverity, subjectType string, subjectID int64, message string, now time.Time) *Alert {
	key := DedupKeyOf(kind, subjectType, subjectID)
	return &Alert{
		DedupKey:    key,
		Kind:        kind,
		Severity:    sev,
		Status:      AlertOpen,
		SubjectType: subjectType,
		SubjectID:   subjectID,
		Message:     message,
		Count:       1,
		FirstSeenAt: now,
		LastSeenAt:  now,
	}
}

// DedupKeyOf 去重键 = kind + 主体类型 + 主体 ID。
func DedupKeyOf(kind, subjectType string, subjectID int64) string {
	var b strings.Builder
	b.WriteString(kind)
	b.WriteByte(':')
	b.WriteString(subjectType)
	b.WriteByte(':')
	fmt.Fprintf(&b, "%d", subjectID)
	return b.String()
}

// Observe 记录一次重复触发：计数 +1、刷新最近时间，必要时升级级别。
func (a *Alert) Observe(sev AlertSeverity, now time.Time) {
	a.Count++
	a.LastSeenAt = now
	if sevRank(sev) > sevRank(a.Severity) {
		a.Severity = sev
	}
}

// Acknowledge 确认告警。
func (a *Alert) Acknowledge(now time.Time) error {
	if !CanAcknowledge(a.Status) {
		return fmt.Errorf("%w: alert %d in %s cannot be acked", ErrInvalidState, a.ID, a.Status)
	}
	t := now
	a.Status = AlertAcked
	a.AckedAt = &t
	return nil
}

// Close 关闭告警。
func (a *Alert) Close(now time.Time) error {
	if !CanClose(a.Status) {
		return fmt.Errorf("%w: alert %d in %s cannot be closed", ErrInvalidState, a.ID, a.Status)
	}
	t := now
	a.Status = AlertClosed
	a.ClosedAt = &t
	return nil
}

func sevRank(s AlertSeverity) int {
	switch s {
	case SevInfo:
		return 0
	case SevWarn:
		return 1
	default:
		return 2
	}
}
