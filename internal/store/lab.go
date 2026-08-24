package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jb843051627/saltern-watch/internal/model"
)

// InsertLabSample 写入化验样本（含品位评分）。
func (d *DB) InsertLabSample(l *model.LabSample) error {
	if err := l.Validate(); err != nil {
		return err
	}
	l.EvaluatePurity()
	res, err := d.db.Exec(
		`INSERT INTO lab_samples(crystallizer_id,taken_at,na_mg_ratio,sulfate_ppm,insoluble_ppm,analyst,purity)
		 VALUES(?,?,?,?,?,?,?)`,
		l.CrystallizerID, l.TakenAt.Unix(), l.NaMgRatio, l.SulfatePPM,
		l.InsolublePPM, l.Analyst, l.Purity)
	if err != nil {
		return wrapInsert("lab sample", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	l.ID = id
	return nil
}

// ListLabSamplesByCryst 结晶池化验历史（时间倒序）。
func (d *DB) ListLabSamplesByCryst(crystID int64, limit int) ([]*model.LabSample, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := d.db.Query(
		`SELECT id,crystallizer_id,taken_at,na_mg_ratio,sulfate_ppm,insoluble_ppm,analyst,purity
		 FROM lab_samples WHERE crystallizer_id=? ORDER BY taken_at DESC LIMIT ?`,
		crystID, limit)
	if err != nil {
		return nil, fmt.Errorf("store: list lab samples: %w", err)
	}
	defer rows.Close()
	out := make([]*model.LabSample, 0, limit)
	for rows.Next() {
		var l model.LabSample
		var at int64
		if err := rows.Scan(&l.ID, &l.CrystallizerID, &at, &l.NaMgRatio,
			&l.SulfatePPM, &l.InsolublePPM, &l.Analyst, &l.Purity); err != nil {
			return nil, fmt.Errorf("store: scan lab sample: %w", err)
		}
		l.TakenAt = time.Unix(at, 0)
		out = append(out, &l)
	}
	return out, rows.Err()
}

// LatestPurity 最新品位，无样本返回 ErrNotFound。
func (d *DB) LatestPurity(crystID int64) (float64, error) {
	var purity sql.NullFloat64
	err := d.db.QueryRow(
		`SELECT purity FROM lab_samples WHERE crystallizer_id=? ORDER BY taken_at DESC LIMIT 1`,
		crystID).Scan(&purity)
	if errors.Is(err, sql.ErrNoRows) || !purity.Valid {
		return 0, model.ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("store: latest purity: %w", err)
	}
	return purity.Float64, nil
}

// InsertSensor 登记传感器。
func (d *DB) InsertSensor(s *model.Sensor) error {
	if err := s.Validate(); err != nil {
		return err
	}
	res, err := d.db.Exec(
		`INSERT INTO sensors(pond_id,kind,model,active,offset_be,updated) VALUES(?,?,?,?,?,?)`,
		s.PondID, s.Kind, s.Model, s.Active, s.OffsetBe, time.Now().Unix())
	if err != nil {
		return wrapInsert("sensor", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	s.ID = id
	s.Updated = time.Now()
	return nil
}

// GetSensor 查询传感器。
func (d *DB) GetSensor(id int64) (*model.Sensor, error) {
	row := d.db.QueryRow(`SELECT id,pond_id,kind,model,active,offset_be,updated FROM sensors WHERE id=?`, id)
	var s model.Sensor
	var updated int64
	err := row.Scan(&s.ID, &s.PondID, &s.Kind, &s.Model, &s.Active, &s.OffsetBe, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: get sensor: %w", err)
	}
	s.Updated = time.Unix(updated, 0)
	return &s, nil
}

// SaveSensorOffset 更新传感器偏移。
func (d *DB) SaveSensorOffset(id int64, offset float64) error {
	res, err := d.db.Exec(`UPDATE sensors SET offset_be=?,updated=? WHERE id=?`,
		offset, time.Now().Unix(), id)
	if err != nil {
		return fmt.Errorf("store: save sensor offset: %w", err)
	}
	return requireAffected(res, model.ErrNotFound)
}

// InsertCalibration 记录校准历史。
func (d *DB) InsertCalibration(c *model.CalibrationRecord) error {
	c.ComputeOffset()
	res, err := d.db.Exec(
		`INSERT INTO calibrations(sensor_id,reference_be,raw_be,offset,created_at) VALUES(?,?,?,?,?)`,
		c.SensorID, c.ReferenceBe, c.RawBe, c.Offset, time.Now().Unix())
	if err != nil {
		return wrapInsert("calibration", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	c.ID = id
	c.CreatedAt = time.Now()
	return nil
}
