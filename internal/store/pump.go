package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jb843051627/saltern-watch/internal/model"
)

// CreatePump 新建泵站。
func (d *DB) CreatePump(p *model.Pump) error {
	if err := p.Validate(); err != nil {
		return err
	}
	var serviceAt int64
	if !p.LastServiceAt.IsZero() {
		serviceAt = p.LastServiceAt.Unix()
	}
	res, err := d.db.Exec(
		`INSERT INTO pumps(name,capacity_m3h,status,hours_run,last_service_at,created_at)
		 VALUES(?,?,?,?,?,?)`,
		p.Name, p.CapacityM3H, string(p.Status), p.HoursRun, serviceAt, time.Now().Unix())
	if err != nil {
		return wrapInsert("pump", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	p.ID = id
	return nil
}

// GetPump 按 ID 查询。
func (d *DB) GetPump(id int64) (*model.Pump, error) {
	row := d.db.QueryRow(
		`SELECT id,name,capacity_m3h,status,hours_run,last_service_at,created_at FROM pumps WHERE id=?`, id)
	return scanPump(row)
}

// ListPumps 列出泵站。
func (d *DB) ListPumps() ([]*model.Pump, error) {
	rows, err := d.db.Query(
		`SELECT id,name,capacity_m3h,status,hours_run,last_service_at,created_at FROM pumps ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("store: list pumps: %w", err)
	}
	defer rows.Close()
	var out []*model.Pump
	for rows.Next() {
		p, err := scanPump(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SavePump 更新泵状态/运行时长/保养时间。
func (d *DB) SavePump(p *model.Pump) error {
	var serviceAt int64
	if !p.LastServiceAt.IsZero() {
		serviceAt = p.LastServiceAt.Unix()
	}
	res, err := d.db.Exec(
		`UPDATE pumps SET name=?,capacity_m3h=?,status=?,hours_run=?,last_service_at=? WHERE id=?`,
		p.Name, p.CapacityM3H, string(p.Status), p.HoursRun, serviceAt, p.ID)
	if err != nil {
		return fmt.Errorf("store: save pump: %w", err)
	}
	return requireAffected(res, model.ErrNotFound)
}

// ListPumpsNeedingService 找出运行时长距保养超阈值的泵。
func (d *DB) ListPumpsNeedingService(sinceHours float64) ([]*model.Pump, error) {
	rows, err := d.db.Query(
		`SELECT id,name,capacity_m3h,status,hours_run,last_service_at,created_at FROM pumps
		 WHERE hours_run >= ? OR (last_service_at > 0 AND hours_run - last_service_at >= ?)`,
		sinceHours, sinceHours)
	if err != nil {
		return nil, fmt.Errorf("store: pumps needing service: %w", err)
	}
	defer rows.Close()
	var out []*model.Pump
	for rows.Next() {
		p, err := scanPump(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func scanPump(row rowScanner) (*model.Pump, error) {
	var p model.Pump
	var status string
	var serviceAt sql.NullInt64
	err := row.Scan(&p.ID, &p.Name, &p.CapacityM3H, &status, &p.HoursRun, &serviceAt, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: scan pump: %w", err)
	}
	p.Status = model.PumpStatus(status)
	if serviceAt.Valid && serviceAt.Int64 > 0 {
		p.LastServiceAt = time.Unix(serviceAt.Int64, 0)
	}
	return &p, nil
}
