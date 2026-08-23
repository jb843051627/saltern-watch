package handler

import (
	"net/http"
	"time"
)

type transferReq struct {
	FromPondID  int64  `json:"from_pond_id"`
	ToPondID    int64  `json:"to_pond_id"`
	PumpID      int64  `json:"pump_id"`
	VolumeM3    float64 `json:"volume_m3"`
	ScheduledAt string `json:"scheduled_at"`
}

func (h *Handler) scheduleTransfer(w http.ResponseWriter, r *http.Request) {
	var req transferReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	scheduledAt := time.Now()
	if req.ScheduledAt != "" {
		if t, err := time.Parse(time.RFC3339, req.ScheduledAt); err == nil {
			scheduledAt = t
		}
	}
	job, err := h.Svc.Transfers.Schedule(req.FromPondID, req.ToPondID, req.PumpID,
		req.VolumeM3, scheduledAt)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusCreated, job)
}

func (h *Handler) cancelTransfer(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, err)
		return
	}
	job, err := h.Svc.Transfers.Cancel(id)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, job)
}
