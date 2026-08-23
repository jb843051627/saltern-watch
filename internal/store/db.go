// Package store 基于 SQLite 的持久化层。
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB 封装 *sql.DB 与公共助手。
type DB struct {
	db   *sql.DB
	path string
}

// Open 打开（必要时创建）SQLite 数据库文件并执行迁移。
func Open(path string) (*DB, error) {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("store: mkdir %s: %w", dir, err)
		}
	}
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	sdb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open sqlite: %w", err)
	}
	// SQLite 单文件写入，串行化避免锁竞争。
	sdb.SetMaxOpenConns(1)
	d := &DB{db: sdb, path: path}
	if err := d.migrate(); err != nil {
		_ = sdb.Close()
		return nil, err
	}
	return d, nil
}

// Close 关闭底层连接。
func (d *DB) Close() error { return d.db.Close() }

// SQL 暴露底层连接（事务助手使用）。
func (d *DB) SQL() *sql.DB { return d.db }

// migrate 建表与索引。
func (d *DB) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS ponds (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			area_m2 REAL NOT NULL,
			stage INTEGER NOT NULL,
			status TEXT NOT NULL,
			brine_level_cm REAL NOT NULL DEFAULT 0,
			target_be REAL NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS brine_readings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			pond_id INTEGER NOT NULL REFERENCES ponds(id),
			taken_at INTEGER NOT NULL,
			be REAL NOT NULL,
			temp_c REAL NOT NULL,
			level_cm REAL NOT NULL,
			source TEXT NOT NULL,
			flag TEXT NOT NULL,
			recorded_at INTEGER NOT NULL,
			UNIQUE(pond_id, taken_at)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_readings_pond_time ON brine_readings(pond_id, taken_at DESC)`,
		`CREATE TABLE IF NOT EXISTS crystallizers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			capacity_tons REAL NOT NULL,
			state TEXT NOT NULL,
			filled_tons REAL NOT NULL DEFAULT 0,
			salinity REAL NOT NULL DEFAULT 0,
			ripened_since INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS harvest_batches (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			crystallizer_id INTEGER NOT NULL REFERENCES crystallizers(id),
			status TEXT NOT NULL,
			tons REAL NOT NULL DEFAULT 0,
			moisture REAL NOT NULL DEFAULT 0,
			grade TEXT NOT NULL DEFAULT '',
			opened_at INTEGER NOT NULL,
			completed_at INTEGER,
			note TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_harvest_cryst ON harvest_batches(crystallizer_id, status)`,
		`CREATE TABLE IF NOT EXISTS pumps (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			capacity_m3h REAL NOT NULL,
			status TEXT NOT NULL,
			hours_run REAL NOT NULL DEFAULT 0,
			last_service_at INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS transfer_jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			from_pond_id INTEGER NOT NULL REFERENCES ponds(id),
			to_pond_id INTEGER NOT NULL REFERENCES ponds(id),
			pump_id INTEGER NOT NULL REFERENCES pumps(id),
			volume_m3 REAL NOT NULL,
			status TEXT NOT NULL,
			scheduled_at INTEGER NOT NULL,
			started_at INTEGER,
			ended_at INTEGER,
			fail_reason TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS weather_samples (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			taken_at INTEGER NOT NULL UNIQUE,
			air_temp_c REAL NOT NULL,
			humidity REAL NOT NULL,
			wind_ms REAL NOT NULL,
			rainfall_mm REAL NOT NULL,
			evap_rate_mm REAL NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS alerts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			dedup_key TEXT NOT NULL UNIQUE,
			kind TEXT NOT NULL,
			severity TEXT NOT NULL,
			status TEXT NOT NULL,
			subject_type TEXT NOT NULL,
			subject_id INTEGER NOT NULL,
			message TEXT NOT NULL,
			count INTEGER NOT NULL DEFAULT 1,
			first_seen_at INTEGER NOT NULL,
			last_seen_at INTEGER NOT NULL,
			acked_at INTEGER,
			closed_at INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS maintenance_tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			target_type TEXT NOT NULL,
			target_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			status TEXT NOT NULL,
			due_at INTEGER NOT NULL,
			done_at INTEGER,
			note TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS event_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			actor TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			entity_id INTEGER NOT NULL,
			action TEXT NOT NULL,
			detail TEXT NOT NULL DEFAULT '',
			occurred_at INTEGER NOT NULL
		)`,
	}
	for _, s := range stmts {
		if _, err := d.db.Exec(s); err != nil {
			return fmt.Errorf("store: migrate: %w", err)
		}
	}
	return nil
}

// nullTime 兼容可空时间列。
func nullTime(ts int64, valid bool) *int64 {
	if !valid {
		return nil
	}
	v := ts
	return &v
}
