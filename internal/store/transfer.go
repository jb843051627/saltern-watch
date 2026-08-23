package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jb843051627/saltern-watch/internal/model"
)

// CreateTransferJob 建立输卤任务。
func (d *DB) CreateTransferJob(t *model.TransferJob) error {
	if err := t.Validate(); err != nil {
		return err
	}
	res, err := d.db.Exec(
		`INSERT INTO transfer_jobs(from_pond_id,to_pond_id,pump_id,volume_m3,status,scheduled_at)
		 VALUES(?,?,?,?,?,?)`,
		t.FromPondID, t.ToPondID, t.PumpID, t.VolumeM3,
		string(t.Status), t.ScheduledAt.Unix())
	if err != nil {
		return wrapInsert("transfer job", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	t.ID = id
	return nil
}

// GetTransferJob 按 ID 查询。
func (d *DB) GetTransferJob(id int64) (*model.TransferJob, error) {
	row := d.db.QueryRow(transferCols+` FROM transfer_jobs WHERE id=?`, id)
	return scanTransfer(row)
}

// PendingTransfers 到期未执行的任务（调度器消费）。
func (d *DB) PendingTransfers(now time.Time) ([]*model.TransferJob, error) {
	rows, err := d.db.Query(
		transferCols+` FROM transfer_jobs WHERE status='pending' AND scheduled_at<=? ORDER BY scheduled_at`,
		now.Unix())
	if err != nil {
		return nil, fmt.Errorf("store: pending transfers: %w", err)
	}
	defer rows.Close()
	var out []*model.TransferJob
	for rows.Next() {
		t, err := scanTransfer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// SaveTransferJob 更新任务状态。
func (d *DB) SaveTransferJob(t *model.TransferJob) error {
	var started, ended any
	if t.StartedAt != nil {
		started = t.StartedAt.Unix()
	}
	if t.EndedAt != nil {
		ended = t.EndedAt.Unix()
	}
	res, err := d.db.Exec(
		`UPDATE transfer_jobs SET status=?,started_at=?,ended_at=?,fail_reason=? WHERE id=?`,
		string(t.Status), started, ended, t.FailReason, t.ID)
	if err != nil {
		return fmt.Errorf("store: save transfer job: %w", err)
	}
	return requireAffected(res, model.ErrNotFound)
}

// ListTransfersByStatus 按状态列任务。
func (d *DB) ListTransfersByStatus(status model.TransferStatus, limit int) ([]*model.TransferJob, error) {
	rows, err := d.db.Query(
		transferCols+` FROM transfer_jobs WHERE status=? ORDER BY scheduled_at DESC LIMIT ?`,
		string(status), limit)
	if err != nil {
		return nil, fmt.Errorf("store: transfers by status: %w", err)
	}
	defer rows.Close()
	out := make([]*model.TransferJob, 0, 8)
	for rows.Next() {
		t, err := scanTransfer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

const transferCols = `SELECT id,from_pond_id,to_pond_id,pump_id,volume_m3,status,scheduled_at,started_at,ended_at,fail_reason`

func scanTransfer(row rowScanner) (*model.TransferJob, error) {
	var t model.TransferJob
	var status string
	var scheduled int64
	var started, ended sql.NullInt64
	err := row.Scan(&t.ID, &t.FromPondID, &t.ToPondID, &t.PumpID, &t.VolumeM3,
		&status, &scheduled, &started, &ended, &t.FailReason)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: scan transfer job: %w", err)
	}
	t.Status = model.TransferStatus(status)
	t.ScheduledAt = time.Unix(scheduled, 0)
	if started.Valid {
		v := time.Unix(started.Int64, 0)
		t.StartedAt = &v
	}
	if ended.Valid {
		v := time.Unix(ended.Int64, 0)
		t.EndedAt = &v
	}
	return &t, nil
}
