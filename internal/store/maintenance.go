package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jb843051627/saltern-watch/internal/model"
)

// CreateMaintenanceTask 建维护任务。
func (d *DB) CreateMaintenanceTask(m *model.MaintenanceTask) error {
	if err := m.Validate(); err != nil {
		return err
	}
	res, err := d.db.Exec(
		`INSERT INTO maintenance_tasks(target_type,target_id,title,status,due_at,note,created_at)
		 VALUES(?,?,?,?,?,?,?)`,
		string(m.TargetType), m.TargetID, m.Title, string(m.Status),
		m.DueAt.Unix(), m.Note, time.Now().Unix())
	if err != nil {
		return wrapInsert("maintenance task", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	m.ID = id
	return nil
}

// GetMaintenanceTask 按 ID 查询。
func (d *DB) GetMaintenanceTask(id int64) (*model.MaintenanceTask, error) {
	row := d.db.QueryRow(taskCols+` FROM maintenance_tasks WHERE id=?`, id)
	return scanTask(row)
}

// SaveMaintenanceTask 更新任务。
func (d *DB) SaveMaintenanceTask(m *model.MaintenanceTask) error {
	var done any
	if m.DoneAt != nil {
		done = m.DoneAt.Unix()
	}
	res, err := d.db.Exec(
		`UPDATE maintenance_tasks SET status=?,done_at=?,note=? WHERE id=?`,
		string(m.Status), done, m.Note, m.ID)
	if err != nil {
		return fmt.Errorf("store: save maintenance task: %w", err)
	}
	return requireAffected(res, model.ErrNotFound)
}

// ListMaintenanceTasksByStatus 按状态列任务。
func (d *DB) ListMaintenanceTasksByStatus(status model.MaintenanceStatus) ([]*model.MaintenanceTask, error) {
	rows, err := d.db.Query(
		taskCols+` FROM maintenance_tasks WHERE status=? ORDER BY due_at`, string(status))
	if err != nil {
		return nil, fmt.Errorf("store: list tasks: %w", err)
	}
	defer rows.Close()
	out := make([]*model.MaintenanceTask, 0, 8)
	for rows.Next() {
		m, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// OverdueMaintenanceTasks 逾期未完结任务。
func (d *DB) OverdueMaintenanceTasks(now time.Time) ([]*model.MaintenanceTask, error) {
	rows, err := d.db.Query(
		taskCols+` FROM maintenance_tasks
		 WHERE status IN ('planned','in_progress') AND due_at < ? ORDER BY due_at`,
		now.Unix())
	if err != nil {
		return nil, fmt.Errorf("store: overdue tasks: %w", err)
	}
	defer rows.Close()
	out := make([]*model.MaintenanceTask, 0, 4)
	for rows.Next() {
		m, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// InsertEvent 写审计事件。
func (d *DB) InsertEvent(e model.EventLog) error {
	_, err := d.db.Exec(
		`INSERT INTO event_logs(actor,entity_type,entity_id,action,detail,occurred_at)
		 VALUES(?,?,?,?,?,?)`,
		e.Actor, e.EntityType, e.EntityID, e.Action, e.Detail, e.OccurredAt.Unix())
	if err != nil {
		return fmt.Errorf("store: insert event: %w", err)
	}
	return nil
}

// RecentEvents 最近审计事件。
func (d *DB) RecentEvents(limit int) ([]model.EventLog, error) {
	rows, err := d.db.Query(
		`SELECT actor,entity_type,entity_id,action,detail,occurred_at FROM (
		    SELECT * FROM event_logs ORDER BY id DESC LIMIT ?
		 ) ORDER BY id`, limit)
	if err != nil {
		return nil, fmt.Errorf("store: recent events: %w", err)
	}
	defer rows.Close()
	out := make([]model.EventLog, 0, limit)
	for rows.Next() {
		var e model.EventLog
		var at int64
		if err := rows.Scan(&e.Actor, &e.EntityType, &e.EntityID, &e.Action, &e.Detail, &at); err != nil {
			return nil, fmt.Errorf("store: scan event: %w", err)
		}
		e.OccurredAt = time.Unix(at, 0)
		out = append(out, e)
	}
	return out, rows.Err()
}

const taskCols = `SELECT id,target_type,target_id,title,status,due_at,done_at,note,created_at`

func scanTask(row rowScanner) (*model.MaintenanceTask, error) {
	var m model.MaintenanceTask
	var targetType, status string
	var due, created int64
	var done sql.NullInt64
	err := row.Scan(&m.ID, &targetType, &m.TargetID, &m.Title, &status, &due, &done, &m.Note, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: scan task: %w", err)
	}
	m.TargetType = model.TargetKind(targetType)
	m.Status = model.MaintenanceStatus(status)
	m.DueAt = time.Unix(due, 0)
	if done.Valid {
		t := time.Unix(done.Int64, 0)
		m.DoneAt = &t
	}
	return &m, nil
}
