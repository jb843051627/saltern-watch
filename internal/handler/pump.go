package handler

import (
	"net/http"
)

type createPumpReq struct {
	Name        string  `json:"name"`
	CapacityM3H float64 `json:"capacity_m3h"`
}

func (h *Handler) createPump(w http.ResponseWriter, r *http.Request) {
	var req createPumpReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	p, err := h.Svc.Pumps.Create(req.Name, req.CapacityM3H)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusCreated, p)
}

func (h *Handler) listPumps(w http.ResponseWriter, _ *http.Request) {
	ps, err := h.Svc.Pumps.List()
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, ps)
}

func (h *Handler) servicePump(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, err)
		return
	}
	p, err := h.Svc.Pumps.Service(id)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, p)
}
