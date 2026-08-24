// Package config 解析服务启动配置。
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config 服务运行配置。
type Config struct {
	HTTPAddr      string
	DBPath        string
	StaticDir     string
	PollInterval  time.Duration
	EvalInterval  time.Duration
	TickInterval  time.Duration
	WorkerCount   int
	LocalZone     *time.Location
	SensorToken   string
	MaxBatchReads int
}

// FromEnv 从环境变量读取配置，缺省值适配本地开发。
func FromEnv() (*Config, error) {
	cfg := &Config{
		HTTPAddr:      envStr("SALTERN_HTTP_ADDR", ":8080"),
		DBPath:        envStr("SALTERN_DB_PATH", "saltern.db"),
		StaticDir:     envStr("SALTERN_STATIC_DIR", "web/static"),
		PollInterval:  envDur("SALTERN_POLL_INTERVAL", 30*time.Second),
		EvalInterval:  envDur("SALTERN_EVAL_INTERVAL", time.Minute),
		TickInterval:  envDur("SALTERN_TICK_INTERVAL", 15*time.Second),
		WorkerCount:   envInt("SALTERN_WORKERS", 2),
		SensorToken:   os.Getenv("SALTERN_SENSOR_TOKEN"),
		MaxBatchReads: envInt("SALTERN_MAX_BATCH", 500),
	}
	zoneName := envStr("SALTERN_TZ", "Asia/Shanghai")
	loc, err := time.LoadLocation(zoneName)
	if err != nil {
		return nil, fmt.Errorf("load timezone %q: %w", zoneName, err)
	}
	cfg.LocalZone = loc
	if cfg.WorkerCount < 1 || cfg.WorkerCount > 8 {
		return nil, fmt.Errorf("worker count %d out of range [1,8]", cfg.WorkerCount)
	}
	if cfg.MaxBatchReads < 1 {
		return nil, fmt.Errorf("max batch reads must be positive")
	}
	return cfg, nil
}

// DSN 返回 SQLite 连接串（纯文件路径，pragma 由 store 层统一附加）。
func (c *Config) DSN() string {
	return c.DBPath
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envDur(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return def
}
