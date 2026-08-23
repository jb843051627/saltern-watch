package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jb843051627/saltern-watch/internal/model"
)

const crystCols = `id,name,capacity_tons,state,filled_tons,salinity,ripened_since,created_at,updated_at`

// CreateCrystallizer 新建结晶池。
func (d *DB) CreateCrystallizer(c *model.Crystallizer) error {
	if err := c.Validate(); err != nil {
		return err
	}
	now := time.Now().Unix()
	res, err := d.db.Exec(
		`INSERT INTO crystallizers(name,capacity_tons,state,filled_tons,salinity,ripened_since,created_at,updated_at)
		 VALUES(?,?,?,?,?,?,?,?)`,
		c.Name, c.CapacityTons, string(c.State), c.FilledTons, c.Salinity, c.RipenedSince, now, now)
	if err != nil {
		return wrapInsert("crystallizer", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	c.ID = id
	c.CreatedAt = now
	c.UpdatedAt = now
	return nil
}

// GetCrystallizer 按 ID 查询。
func (d *DB) GetCrystallizer(id int64) (*model.Crystallizer, error) {
	row := d.db.QueryRow(`SELECT `+crystCols+` FROM crystallizers WHERE id=?`, id)
	return scanCryst(row)
}

// ListCrystallizers 列出全部结晶池。
func (d *DB) ListCrystallizers() ([]*model.Crystallizer, error) {
	rows, err := d.db.Query(`SELECT ` + crystCols + ` FROM crystallizers ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("store: list crystallizers: %w", err)
	}
	defer rows.Close()
	var out []*model.Crystallizer
	for rows.Next() {
		c, err := scanCryst(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SaveCrystallizer 全量更新状态字段（乐观并发：state 由服务层校验）。
func (d *DB) SaveCrystallizer(c *model.Crystallizer) error {
	res, err := d.db.Exec(
		`UPDATE crystallizers SET name=?,capacity_tons=?,state=?,filled_tons=?,salinity=?,ripened_since=?,updated_at=?
		 WHERE id=?`,
		c.Name, c.CapacityTons, string(c.State), c.FilledTons, c.Salinity,
		c.RipenedSince, time.Now().Unix(), c.ID)
	if err != nil {
		return fmt.Errorf("store: save crystallizer: %w", err)
	}
	return requireAffected(res, model.ErrNotFound)
}

// CountCrystallizersByState 按状态统计数量。
func (d *DB) CountCrystallizersByState() (map[model.CrystState]int, error) {
	rows, err := d.db.Query(`SELECT state,COUNT(*) FROM crystallizers GROUP BY state`)
	if err != nil {
		return nil, fmt.Errorf("store: count by state: %w", err)
	}
	defer rows.Close()
	out := map[model.CrystState]int{}
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			return nil, fmt.Errorf("store: scan count: %w", err)
		}
		out[model.CrystState(st)] = n
	}
	return out, rows.Err()
}

func scanCryst(row rowScanner) (*model.Crystallizer, error) {
	var c model.Crystallizer
	var state string
	err := row.Scan(&c.ID, &c.Name, &c.CapacityTons, &state, &c.FilledTons,
		&c.Salinity, &c.RipenedSince, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: scan crystallizer: %w", err)
	}
	c.State = model.CrystState(state)
	return &c, nil
}
