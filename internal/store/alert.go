package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jb843051627/saltern-watch/internal/model"
)

// UpsertAlert 按去重键聚合告警：已存在则计数+刷新，否则新建。
func (d *DB) UpsertAlert(a *model.Alert) error {
	res, err := d.db.Exec(
		`INSERT INTO alerts(dedup_key,kind,severity,status,subject_type,subject_id,message,count,first_seen_at,last_seen_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?)
		 ON CONFLICT(dedup_key) DO UPDATE SET
		   count=alerts.count+1,
		   severity=excluded.severity,
		   last_seen_at=excluded.last_seen_at,
		   message=excluded.message`,
		a.DedupKey, a.Kind, string(a.Severity), string(a.Status),
		a.SubjectType, a.SubjectID, a.Message, a.Count,
		a.FirstSeenAt.Unix(), a.LastSeenAt.Unix())
	if err != nil {
		return wrapInsert("alert", err)
	}
	if id, err := res.LastInsertId(); err == nil {
		a.ID = id
	}
	return d.hydrateAlert(a)
}

// hydrateAlert 回读数据库中的聚合结果填充 ID/Count。
func (d *DB) hydrateAlert(a *model.Alert) error {
	row := d.db.QueryRow(
		`SELECT id,count,status FROM alerts WHERE dedup_key=?`, a.DedupKey)
	var status string
	if err := row.Scan(&a.ID, &a.Count, &status); err != nil {
		return fmt.Errorf("store: hydrate alert: %w", err)
	}
	a.Status = model.AlertStatus(status)
	return nil
}

// GetAlert 按 ID 查询。
func (d *DB) GetAlert(id int64) (*model.Alert, error) {
	row := d.db.QueryRow(alertCols+` FROM alerts WHERE id=?`, id)
	a, err := scanAlert(row)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// FindOpenByDedupKey 查询未关闭告警。
func (d *DB) FindOpenByDedupKey(key string) (*model.Alert, error) {
	row := d.db.QueryRow(alertCols+` FROM alerts WHERE dedup_key=? AND status!='closed'`, key)
	a, err := scanAlert(row)
	if errors.Is(err, model.ErrNotFound) {
		return nil, model.ErrNotFound
	}
	return a, err
}

// ListAlertsByStatus 列出指定状态告警（可选 limit<=0 不限）。
func (d *DB) ListAlertsByStatus(status model.AlertStatus, limit int) ([]*model.Alert, error) {
	q := alertCols + ` FROM alerts WHERE status=? ORDER BY last_seen_at DESC`
	args := []any{string(status)}
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list alerts: %w", err)
	}
	defer rows.Close()
	out := make([]*model.Alert, 0, 8)
	for rows.Next() {
		a, err := scanAlert(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// UpdateAlertStatus 更新告警状态与时间戳。
func (d *DB) UpdateAlertStatus(id int64, status model.AlertStatus, ts time.Time) error {
	var acked, closed any
	switch status {
	case model.AlertAcked:
		acked = ts.Unix()
	case model.AlertClosed:
		closed = ts.Unix()
	}
	res, err := d.db.Exec(
		`UPDATE alerts SET status=?,acked_at=COALESCE(?,acked_at),closed_at=COALESCE(?,closed_at) WHERE id=?`,
		string(status), acked, closed, id)
	if err != nil {
		return fmt.Errorf("store: update alert status: %w", err)
	}
	return requireAffected(res, model.ErrNotFound)
}

// CountActiveAlerts 未关闭告警数。
func (d *DB) CountActiveAlerts() (int, error) {
	var n int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM alerts WHERE status!='closed'`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("store: count active alerts: %w", err)
	}
	return n, nil
}

const alertCols = `SELECT id,dedup_key,kind,severity,status,subject_type,subject_id,message,count,first_seen_at,last_seen_at,acked_at,closed_at`

func scanAlert(row rowScanner) (*model.Alert, error) {
	var a model.Alert
	var sev, status string
	var first, last int64
	var acked, closed sql.NullInt64
	err := row.Scan(&a.ID, &a.DedupKey, &a.Kind, &sev, &status, &a.SubjectType,
		&a.SubjectID, &a.Message, &a.Count, &first, &last, &acked, &closed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: scan alert: %w", err)
	}
	a.Severity = model.AlertSeverity(sev)
	a.Status = model.AlertStatus(status)
	a.FirstSeenAt = time.Unix(first, 0)
	a.LastSeenAt = time.Unix(last, 0)
	if acked.Valid {
		t := time.Unix(acked.Int64, 0)
		a.AckedAt = &t
	}
	if closed.Valid {
		t := time.Unix(closed.Int64, 0)
		a.ClosedAt = &t
	}
	return &a, nil
}
