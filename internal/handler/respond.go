// Package handler 提供 HTTP API。
package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jb843051627/saltern-watch/internal/model"
)

// respond JSON 输出。
func respond(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// fail 统一错误映射：领域哨兵 → HTTP 状态码。
func fail(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	switch {
	case errors.Is(err, model.ErrNotFound):
		code = http.StatusNotFound
	case errors.Is(err, model.ErrInvalidInput):
		code = http.StatusBadRequest
	case errors.Is(err, model.ErrDuplicate):
		code = http.StatusConflict
	case errors.Is(err, model.ErrInvalidState),
		errors.Is(err, model.ErrPondNotReady),
		errors.Is(err, model.ErrCrystFull),
		errors.Is(err, model.ErrHarvestOpen),
		errors.Is(err, model.ErrPumpUnavailable):
		code = http.StatusUnprocessableEntity
	}
	respond(w, code, map[string]string{"error": err.Error()})
}

// decode 请求体解析。
func decode(r *http.Request, v any) error {
	if r.Body == nil {
		return model.ErrInvalidInput
	}
	d := json.NewDecoder(r.Body)
	if err := d.Decode(v); err != nil {
		return errors.Join(model.ErrInvalidInput, err)
	}
	return nil
}
