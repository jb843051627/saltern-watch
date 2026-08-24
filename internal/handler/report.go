package handler

import (
	"net/http"
	"time"
)

func (h *Handler) dailyReport(w http.ResponseWriter, r *http.Request) {
	day := queryTime(r, "day", time.Now().AddDate(0, 0, -1))
	data, err := h.Svc.Reports.DailyCSV(day)
	if err != nil {
		fail(w, err)
		return
	}
	name := "saltern-daily-" + day.Format("20060102") + ".csv"
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename="+name)
	_, _ = w.Write(data)
}

func (h *Handler) dashboard(w http.ResponseWriter, _ *http.Request) {
	snap, err := h.Dashboard.Snapshot()
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, snap)
}
