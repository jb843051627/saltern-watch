package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jb843051627/saltern-watch/internal/model"
)

// InsertReading 写入卤水读数；同池同刻重复返回 ErrDuplicate。
func (d *DB) InsertReading(r *model.BrineReading) error {
	res, err := d.db.Exec(
		`INSERT INTO brine_readings(pond_id,taken_at,be,temp_c,level_cm,source,flag,recorded_at)
		 VALUES(?,?,?,?,?,?,?,?)`,
		r.PondID, r.TakenAt.Unix(), r.Be, r.TempC, r.LevelCm,
		r.Source, string(r.Flag), time.Now().Unix())
	if err != nil {
		return wrapInsert("reading", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	r.ID = id
	return nil
}

// GetReading 单条查询。
func (d *DB) GetReading(id int64) (*model.BrineReading, error) {
	row := d.db.QueryRow(
		`SELECT id,pond_id,taken_at,be,temp_c,level_cm,source,flag,recorded_at
		 FROM brine_readings WHERE id=?`, id)
	var r model.BrineReading
	var taken, recorded int64
	var flag string
	err := row.Scan(&r.ID, &r.PondID, &taken, &r.Be, &r.TempC, &r.LevelCm, &r.Source, &flag, &recorded)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: get reading: %w", err)
	}
	r.TakenAt = time.Unix(taken, 0)
	r.RecordedAt = time.Unix(recorded, 0)
	r.Flag = model.ReadingFlag(flag)
	return &r, nil
}

// LatestReading 查询池最新一条读数，无读数返回 ErrNotFound。
func (d *DB) LatestReading(pondID int64) (*model.BrineReading, error) {
	row := d.db.QueryRow(
		`SELECT id,pond_id,taken_at,be,temp_c,level_cm,source,flag,recorded_at
		 FROM brine_readings WHERE pond_id=? ORDER BY taken_at DESC LIMIT 1`, pondID)
	var r model.BrineReading
	var taken, recorded int64
	var flag string
	err := row.Scan(&r.ID, &r.PondID, &taken, &r.Be, &r.TempC, &r.LevelCm, &r.Source, &flag, &recorded)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: latest reading: %w", err)
	}
	r.TakenAt = time.Unix(taken, 0)
	r.RecordedAt = time.Unix(recorded, 0)
	r.Flag = model.ReadingFlag(flag)
	return &r, nil
}

// ListReadings 按池与时间窗倒序列出读数。
func (d *DB) ListReadings(pondID int64, from, to time.Time, limit int) ([]*model.BrineReading, error) {
	rows, err := d.db.Query(
		`SELECT id,pond_id,taken_at,be,temp_c,level_cm,source,flag,recorded_at
		 FROM brine_readings
		 WHERE pond_id=? AND taken_at>=? AND taken_at<=?
		 ORDER BY taken_at DESC LIMIT ?`,
		pondID, from.Unix(), to.Unix(), limit)
	if err != nil {
		return nil, fmt.Errorf("store: list readings: %w", err)
	}
	defer rows.Close()
	out := make([]*model.BrineReading, 0, 16)
	for rows.Next() {
		var r model.BrineReading
		var taken, recorded int64
		var flag string
		if err := rows.Scan(&r.ID, &r.PondID, &taken, &r.Be, &r.TempC, &r.LevelCm, &r.Source, &flag, &recorded); err != nil {
			return nil, fmt.Errorf("store: scan reading: %w", err)
		}
		r.TakenAt = time.Unix(taken, 0)
		r.RecordedAt = time.Unix(recorded, 0)
		r.Flag = model.ReadingFlag(flag)
		out = append(out, &r)
	}
	return out, rows.Err()
}

// AverageBe 时间窗内平均浓度（无数据返回 NaN 由调用方处理）。
func (d *DB) AverageBe(pondID int64, from, to time.Time) (float64, error) {
	var avg sql.NullFloat64
	err := d.db.QueryRow(
		`SELECT AVG(be) FROM brine_readings WHERE pond_id=? AND taken_at BETWEEN ? AND ? AND flag='ok'`,
		pondID, from.Unix(), to.Unix()).Scan(&avg)
	if err != nil {
		return 0, fmt.Errorf("store: avg be: %w", err)
	}
	if !avg.Valid {
		return 0, model.ErrNotFound
	}
	return avg.Float64, nil
}

// CountRejected 统计窗口内被拒读数。
func (d *DB) CountRejected(from, to time.Time) (int, error) {
	var n int
	err := d.db.QueryRow(
		`SELECT COUNT(*) FROM brine_readings WHERE flag='rejected' AND taken_at BETWEEN ? AND ?`,
		from.Unix(), to.Unix()).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("store: count rejected: %w", err)
	}
	return n, nil
}
