package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/jb843051627/saltern-watch/internal/model"
)

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, model.ErrInvalidInput
	}
	return id, nil
}

func queryTime(r *http.Request, key string, def time.Time) time.Time {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, v); err == nil {
			return t
		}
	}
	return def
}

type createPondReq struct {
	Name     string  `json:"name"`
	AreaM2   float64 `json:"area_m2"`
	Stage    int     `json:"stage"`
	TargetBe float64 `json:"target_be"`
	LevelCm  float64 `json:"level_cm"`
}

func (h *Handler) createPond(w http.ResponseWriter, r *http.Request) {
	var req createPondReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	p, err := h.Svc.Ponds.Create(req.Name, req.AreaM2, req.Stage, req.TargetBe, req.LevelCm)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusCreated, p)
}

func (h *Handler) listPonds(w http.ResponseWriter, _ *http.Request) {
	ps, err := h.Svc.Ponds.List()
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, ps)
}

func (h *Handler) getPond(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, err)
		return
	}
	p, err := h.Svc.Ponds.Get(id)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, p)
}

func (h *Handler) advancePond(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, err)
		return
	}
	beSvc := h.Svc.Brine
	be, beErr := beSvc.CurrentBe(id)
	if beErr != nil {
		fail(w, beErr)
		return
	}
	p, err := h.Svc.Ponds.AdvanceStage(id, be)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, p)
}
