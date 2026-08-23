package handler

import (
	"net/http"
	"time"

	"github.com/jb843051627/saltern-watch/internal/service"
)

type openHarvestReq struct {
	CrystallizerID int64  `json:"crystallizer_id"`
	Note           string `json:"note"`
}

func (h *Handler) openHarvest(w http.ResponseWriter, r *http.Request) {
	var req openHarvestReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	batch, err := h.Svc.Harvests.Open(req.CrystallizerID, req.Note)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusCreated, batch)
}

type completeHarvestReq struct {
	Tons     float64 `json:"tons"`
	Moisture float64 `json:"moisture"`
}

func (h *Handler) completeHarvest(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, err)
		return
	}
	var req completeHarvestReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	batch, err := h.Svc.Harvests.Complete(id, req.Tons, req.Moisture)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, batch)
}

func (h *Handler) monthlyHarvests(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	from := queryTime(r, "from", now.AddDate(0, -3, 0))
	to := queryTime(r, "to", now)
	totals, err := h.Svc.Harvests.MonthlyTotals(service.NewTimeRange(from, to))
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, totals)
}
