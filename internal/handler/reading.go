package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jb843051627/saltern-watch/internal/model"
)

type readingReq struct {
	Be      float64 `json:"be"`
	TempC   float64 `json:"temp_c"`
	LevelCm float64 `json:"level_cm"`
	Source  string  `json:"source"`
	TakenAt string  `json:"taken_at"`
}

func (h *Handler) postReading(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, err)
		return
	}
	var req readingReq
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
	src := req.Source
	if i := strings.LastIndex(src, ":"); i >= 0 {
		src = src[:i] + ":0"
	}
	rd, err := h.Svc.Brine.Ingest(id, req.Be, req.TempC, req.LevelCm, src, takenAt)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusCreated, rd)
}

func (h *Handler) listReadings(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, err)
		return
	}
	now := time.Now()
	from := queryTime(r, "from", now.AddDate(0, 0, -7))
	to := queryTime(r, "to", now)
	limit := 200
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	items, err := h.Svc.Brine.History(id, from, to, limit)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, items)
}

type batchReadingsReq struct {
	Items  []readingReq `json:"items"`
	PondID int64        `json:"pond_id"`
}

func (h *Handler) batchReadings(w http.ResponseWriter, r *http.Request) {
	var req batchReadingsReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	items := make([]*model.BrineReading, 0, len(req.Items))
	for _, it := range req.Items {
		takenAt := time.Now()
		if it.TakenAt != "" {
			if t, err := time.Parse(time.RFC3339, it.TakenAt); err == nil {
				takenAt = t
			}
		}
		items = append(items, &model.BrineReading{
			PondID: req.PondID, Be: it.Be, TempC: it.TempC,
			LevelCm: it.LevelCm, Source: it.Source, TakenAt: takenAt,
		})
	}
	n, err := h.Svc.Brine.BatchIngest(r.Context(), items)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]int{"accepted": n})
}
