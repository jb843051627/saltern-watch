package handler

import (
	"net/http"
)

func (h *Handler) listAlerts(w http.ResponseWriter, r *http.Request) {
	limit := 50
	alerts, err := h.Svc.Alerts.OpenList(limit)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, alerts)
}

func (h *Handler) ackAlert(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, err)
		return
	}
	a, err := h.Svc.Alerts.Acknowledge(id)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, a)
}

func (h *Handler) closeAlert(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, err)
		return
	}
	a, err := h.Svc.Alerts.Close(id)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, a)
}
