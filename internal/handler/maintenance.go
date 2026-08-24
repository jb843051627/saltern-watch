package handler

import (
	"net/http"
	"time"

	"github.com/jb843051627/saltern-watch/internal/model"
)

type planMaintenanceReq struct {
	TargetType string `json:"target_type"`
	TargetID   int64  `json:"target_id"`
	Title      string `json:"title"`
	DueAt      string `json:"due_at"`
}

func (h *Handler) planMaintenance(w http.ResponseWriter, r *http.Request) {
	var req planMaintenanceReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	var kind model.TargetKind
	switch model.TargetKind(req.TargetType) {
	case model.TargetPump:
		kind = model.TargetPump
	case model.TargetPond:
		kind = model.TargetPond
	case model.TargetSensor:
		kind = model.TargetSensor
	default:
		fail(w, model.ErrInvalidInput)
		return
	}
	dueAt := time.Now().AddDate(0, 0, 7)
	if req.DueAt != "" {
		if t, err := time.Parse(time.RFC3339, req.DueAt); err == nil {
			dueAt = t
		}
	}
	m, err := h.Svc.Maintenance.Plan(kind, req.TargetID, req.Title, dueAt)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusCreated, m)
}

func (h *Handler) completeMaintenance(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, err)
		return
	}
	m, err := h.Svc.Maintenance.Complete(id)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, m)
}
