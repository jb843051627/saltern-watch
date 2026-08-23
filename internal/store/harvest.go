package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jb843051627/saltern-watch/internal/model"
)

// CreateHarvestBatch 开批。
func (d *DB) CreateHarvestBatch(h *model.HarvestBatch) error {
	if err := h.Validate(); err != nil {
		return err
	}
	res, err := d.db.Exec(
		`INSERT INTO harvest_batches(crystallizer_id,status,tons,moisture,grade,opened_at,note)
		 VALUES(?,?,?,?,?,?,?)`,
		h.CrystallizerID, string(h.Status), h.Tons, h.Moisture,
		string(h.Grade), h.OpenedAt.Unix(), h.Note)
	if err != nil {
		return wrapInsert("harvest batch", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	h.ID = id
	return nil
}

// GetHarvestBatch 按 ID 查询。
func (d *DB) GetHarvestBatch(id int64) (*model.HarvestBatch, error) {
	row := d.db.QueryRow(
		`SELECT id,crystallizer_id,status,tons,moisture,grade,opened_at,completed_at,note
		 FROM harvest_batches WHERE id=?`, id)
	return scanHarvest(row)
}

// OpenBatchByCryst 查询结晶池未完结批次，无则返回 ErrNotFound。
func (d *DB) OpenBatchByCryst(crystID int64) (*model.HarvestBatch, error) {
	row := d.db.QueryRow(
		`SELECT id,crystallizer_id,status,tons,moisture,grade,opened_at,completed_at,note
		 FROM harvest_batches WHERE crystallizer_id=? AND status='open' LIMIT 1`, crystID)
	h, err := scanHarvest(row)
	if errors.Is(err, model.ErrNotFound) {
		return nil, model.ErrNotFound
	}
	return h, err
}

// SaveHarvestBatch 更新批次。
func (d *DB) SaveHarvestBatch(h *model.HarvestBatch) error {
	var completed any
	if h.CompletedAt != nil {
		completed = h.CompletedAt.Unix()
	}
	if h.Grade == "" && h.Status == model.BatchCompleted {
		h.Grade = model.GradeOf(h.Moisture, h.Tons)
	}
	res, err := d.db.Exec(
		`UPDATE harvest_batches SET status=?,tons=?,moisture=?,grade=?,completed_at=?,note=? WHERE id=?`,
		string(h.Status), h.Tons, h.Moisture, string(h.Grade), completed, h.Note, h.ID)
	if err != nil {
		return fmt.Errorf("store: save harvest batch: %w", err)
	}
	return requireAffected(res, model.ErrNotFound)
}

// ListHarvestBatches 时间窗内批次列表。
func (d *DB) ListHarvestBatches(from, to time.Time) ([]*model.HarvestBatch, error) {
	rows, err := d.db.Query(
		`SELECT id,crystallizer_id,status,tons,moisture,grade,opened_at,completed_at,note
		 FROM harvest_batches WHERE opened_at BETWEEN ? AND ? ORDER BY opened_at`,
		from.Unix(), to.Unix())
	if err != nil {
		return nil, fmt.Errorf("store: list harvest batches: %w", err)
	}
	defer rows.Close()
	out := make([]*model.HarvestBatch, 0, 8)
	for rows.Next() {
		h, err := scanHarvest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// TotalTonsByGrade 统计时间窗各等级产量。
func (d *DB) TotalTonsByGrade(from, to time.Time) (map[model.HarvestGrade]float64, error) {
	rows, err := d.db.Query(
		`SELECT grade,SUM(tons) FROM harvest_batches
		 WHERE status='completed' AND opened_at BETWEEN ? AND ? GROUP BY grade`,
		from.Unix(), to.Unix())
	if err != nil {
		return nil, fmt.Errorf("store: tons by grade: %w", err)
	}
	defer rows.Close()
	out := map[model.HarvestGrade]float64{}
	for rows.Next() {
		var g string
		var tons sql.NullFloat64
		if err := rows.Scan(&g, &tons); err != nil {
			return nil, fmt.Errorf("store: scan tons: %w", err)
		}
		if tons.Valid {
			out[model.HarvestGrade(g)] = tons.Float64
		}
	}
	return out, rows.Err()
}

func scanHarvest(row rowScanner) (*model.HarvestBatch, error) {
	var h model.HarvestBatch
	var status, grade string
	var opened int64
	var completed sql.NullInt64
	err := row.Scan(&h.ID, &h.CrystallizerID, &status, &h.Tons, &h.Moisture,
		&grade, &opened, &completed, &h.Note)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: scan harvest batch: %w", err)
	}
	h.Status = model.BatchStatus(status)
	h.Grade = model.HarvestGrade(grade)
	h.OpenedAt = time.Unix(opened, 0)
	if completed.Valid {
		t := time.Unix(completed.Int64, 0)
		h.CompletedAt = &t
	}
	return &h, nil
}
