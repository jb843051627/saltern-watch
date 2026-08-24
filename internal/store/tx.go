package store

import (
	"context"
	"database/sql"
	"fmt"
)

// WithTx 在事务中执行 fn；fn 返回错误则回滚。
func (d *DB) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin tx: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit tx: %w", err)
	}
	return nil
}

// Ping 健康检查。
func (d *DB) Ping() error { return d.db.Ping() }
