package handler

import (
	"net/http"

	"github.com/jb843051627/saltern-watch/internal/model"
)

type createCrystReq struct {
	Name         string  `json:"name"`
	CapacityTons float64 `json:"capacity_tons"`
}

func (h *Handler) createCrystallizer(w http.ResponseWriter, r *http.Request) {
	var req createCrystReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	c, err := h.Svc.Crystallizers.Create(req.Name, req.CapacityTons)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusCreated, c)
}

func (h *Handler) listCrystallizers(w http.ResponseWriter, _ *http.Request) {
	cs, err := h.Svc.Crystallizers.List()
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, cs)
}

type fillReq struct {
	Tons     float64 `json:"tons"`
	Salinity float64 `json:"salinity"`
}

func (h *Handler) fillCrystallizer(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, err)
		return
	}
	var req fillReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	c, err := h.Svc.Crystallizers.FillBrine(id, req.Tons, req.Salinity)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, c)
}

type transitionReq struct {
	To string `json:"to"`
}

func (h *Handler) transitionCrystallizer(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, err)
		return
	}
	var req transitionReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	to := model.CrystState(req.To)
	switch to {
	case model.CrystEmpty, model.CrystFilling, model.CrystRipening,
		model.CrystHarvestReady, model.CrystHarvesting:
	default:
		fail(w, model.ErrInvalidInput)
		return
	}
	c, err := h.Svc.Crystallizers.Transition(id, to)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, c)
}
