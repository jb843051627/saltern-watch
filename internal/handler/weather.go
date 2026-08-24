package handler

import (
	"net/http"
	"time"
)

type weatherReq struct {
	AirTempC   float64 `json:"air_temp_c"`
	Humidity   float64 `json:"humidity"`
	WindMS     float64 `json:"wind_ms"`
	RainfallMM float64 `json:"rainfall_mm"`
	TakenAt    string  `json:"taken_at"`
}

func (h *Handler) ingestWeather(w http.ResponseWriter, r *http.Request) {
	var req weatherReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	takenAt := time.Now()
	if req.TakenAt != "" {
		if t, err := time.Parse(time.RFC3339, req.TakenAt); err == nil {
			takenAt = t
		}
	}
	ws, err := h.Svc.Weather.Ingest(req.AirTempC, req.Humidity, req.WindMS, req.RainfallMM, takenAt)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusCreated, ws)
}

func (h *Handler) latestWeather(w http.ResponseWriter, _ *http.Request) {
	ws, err := h.Svc.Weather.Latest()
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, ws)
}
