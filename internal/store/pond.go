package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jb843051627/saltern-watch/internal/model"
)

// CreatePond 新建蒸发池。
func (d *DB) CreatePond(p *model.Pond) error {
	if err := p.Validate(); err != nil {
		return err
	}
	now := time.Now().Unix()
	res, err := d.db.Exec(
		`INSERT INTO ponds(name,area_m2,stage,status,brine_level_cm,target_be,created_at,updated_at)
		 VALUES(?,?,?,?,?,?,?,?)`,
		p.Name, p.AreaM2, p.Stage, string(p.Status), p.BrineLevelCm, p.TargetBe, now, now)
	if err != nil {
		return wrapInsert("pond", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	p.ID = id
	p.CreatedAt = now
	p.UpdatedAt = now
	return nil
}

// GetPond 按 ID 查询，未命中返回 model.ErrNotFound。
func (d *DB) GetPond(id int64) (*model.Pond, error) {
	row := d.db.QueryRow(
		`SELECT id,name,area_m2,stage,status,brine_level_cm,target_be,created_at,updated_at
		 FROM ponds WHERE id=?`, id)
	return scanPond(row)
}

// ListPonds 列出全部蒸发池（按 stage、name 排序）。
func (d *DB) ListPonds() ([]*model.Pond, error) {
	rows, err := d.db.Query(
		`SELECT id,name,area_m2,stage,status,brine_level_cm,target_be,created_at,updated_at
		 FROM ponds ORDER BY stage,name`)
	if err != nil {
		return nil, fmt.Errorf("store: list ponds: %w", err)
	}
	defer rows.Close()
	var out []*model.Pond
	for rows.Next() {
		p, err := scanPond(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// UpdatePondLevel 更新液位与状态。
func (d *DB) UpdatePondLevel(id int64, levelCm float64, status model.PondStatus) error {
	res, err := d.db.Exec(
		`UPDATE ponds SET brine_level_cm=?,status=?,updated_at=? WHERE id=?`,
		levelCm, string(status), time.Now().Unix(), id)
	if err != nil {
		return fmt.Errorf("store: update pond level: %w", err)
	}
	return requireAffected(res, model.ErrNotFound)
}

// UpdatePondStage 推进池阶段。
func (d *DB) UpdatePondStage(id int64, stage int, targetBe float64) error {
	res, err := d.db.Exec(
		`UPDATE ponds SET stage=?,target_be=?,updated_at=? WHERE id=?`,
		stage, targetBe, time.Now().Unix(), id)
	if err != nil {
		return fmt.Errorf("store: update pond stage: %w", err)
	}
	return requireAffected(res, model.ErrNotFound)
}

type rowScanner interface{ Scan(dest ...any) error }

func scanPond(row rowScanner) (*model.Pond, error) {
	var p model.Pond
	var status string
	err := row.Scan(&p.ID, &p.Name, &p.AreaM2, &p.Stage, &status,
		&p.BrineLevelCm, &p.TargetBe, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: scan pond: %w", err)
	}
	p.Status = model.PondStatus(status)
	return &p, nil
}

// wrapInsert 统一插入错误语义：唯一冲突 → ErrDuplicate。
func wrapInsert(entity string, err error) error {
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return fmt.Errorf("%w: %s", model.ErrDuplicate, entity)
	}
	return fmt.Errorf("store: insert %s: %w", entity, err)
}

// requireAffected 检查 UPDATE/DELETE 影响行数，0 行返回 sentinel。
func requireAffected(res sql.Result, sentinel error) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sentinel
	}
	return nil
}
