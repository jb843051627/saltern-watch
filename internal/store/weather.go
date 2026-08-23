package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jb843051627/saltern-watch/internal/model"
)

// InsertWeatherSample 写入气象样本（同刻重复覆盖更新）。
func (d *DB) InsertWeatherSample(w *model.WeatherSample) error {
	res, err := d.db.Exec(
		`INSERT INTO weather_samples(taken_at,air_temp_c,humidity,wind_ms,rainfall_mm,evap_rate_mm)
		 VALUES(?,?,?,?,?,?)
		 ON CONFLICT(taken_at) DO UPDATE SET
		   air_temp_c=excluded.air_temp_c,
		   humidity=excluded.humidity,
		   wind_ms=excluded.wind_ms,
		   rainfall_mm=excluded.rainfall_mm,
		   evap_rate_mm=excluded.evap_rate_mm`,
		w.TakenAt.Unix(), w.AirTempC, w.Humidity, w.WindMS, w.RainfallMM, w.EvapRateMM)
	if err != nil {
		return wrapInsert("weather sample", err)
	}
	if id, err := res.LastInsertId(); err == nil && id > 0 {
		w.ID = id
	}
	return nil
}

// RecentWeather 最近 N 条气象样本（时间升序）。
func (d *DB) RecentWeather(limit int) ([]*model.WeatherSample, error) {
	rows, err := d.db.Query(
		`SELECT id,taken_at,air_temp_c,humidity,wind_ms,rainfall_mm,evap_rate_mm FROM (
		    SELECT * FROM weather_samples ORDER BY taken_at DESC LIMIT ?
		 ) ORDER BY taken_at`, limit)
	if err != nil {
		return nil, fmt.Errorf("store: recent weather: %w", err)
	}
	defer rows.Close()
	out := make([]*model.WeatherSample, 0, limit)
	for rows.Next() {
		var w model.WeatherSample
		var taken int64
		if err := rows.Scan(&w.ID, &taken, &w.AirTempC, &w.Humidity, &w.WindMS, &w.RainfallMM, &w.EvapRateMM); err != nil {
			return nil, fmt.Errorf("store: scan weather: %w", err)
		}
		w.TakenAt = time.Unix(taken, 0)
		out = append(out, &w)
	}
	return out, rows.Err()
}

// LatestWeather 最新一条，无数据返回 ErrNotFound。
func (d *DB) LatestWeather() (*model.WeatherSample, error) {
	row := d.db.QueryRow(
		`SELECT id,taken_at,air_temp_c,humidity,wind_ms,rainfall_mm,evap_rate_mm
		 FROM weather_samples ORDER BY taken_at DESC LIMIT 1`)
	var w model.WeatherSample
	var taken int64
	err := row.Scan(&w.ID, &taken, &w.AirTempC, &w.Humidity, &w.WindMS, &w.RainfallMM, &w.EvapRateMM)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: latest weather: %w", err)
	}
	w.TakenAt = time.Unix(taken, 0)
	return &w, nil
}

// AvgEvapRate 窗口内平均蒸发率。
func (d *DB) AvgEvapRate(from, to time.Time) (float64, error) {
	var avg sql.NullFloat64
	err := d.db.QueryRow(
		`SELECT AVG(evap_rate_mm) FROM weather_samples WHERE taken_at BETWEEN ? AND ?`,
		from.Unix(), to.Unix()).Scan(&avg)
	if err != nil {
		return 0, fmt.Errorf("store: avg evap: %w", err)
	}
	if !avg.Valid {
		return 0, model.ErrNotFound
	}
	return avg.Float64, nil
}
