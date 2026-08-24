package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/jb843051627/saltern-watch/internal/service"
)

// Handler 聚合全部路由依赖。
type Handler struct {
	Svc       *service.Service
	Dashboard *service.DashboardService
	staticDir string
	mux       *http.ServeMux
}

// New 构造并注册路由。
func New(svc *service.Service, dash *service.DashboardService, staticDir string) *Handler {
	h := &Handler{Svc: svc, Dashboard: dash, staticDir: staticDir, mux: http.NewServeMux()}
	h.routes()
	return h
}

func (h *Handler) routes() {
	m := h.mux
	m.HandleFunc("GET /api/v1/health", h.health)
	m.HandleFunc("GET /api/v1/dashboard", h.dashboard)

	m.HandleFunc("POST /api/v1/ponds", h.createPond)
	m.HandleFunc("GET /api/v1/ponds", h.listPonds)
	m.HandleFunc("GET /api/v1/ponds/{id}", h.getPond)
	m.HandleFunc("POST /api/v1/ponds/{id}/readings", h.postReading)
	m.HandleFunc("GET /api/v1/ponds/{id}/readings", h.listReadings)
	m.HandleFunc("POST /api/v1/readings/batch", h.batchReadings)
	m.HandleFunc("POST /api/v1/ponds/{id}/advance", h.advancePond)

	m.HandleFunc("POST /api/v1/crystallizers", h.createCrystallizer)
	m.HandleFunc("GET /api/v1/crystallizers", h.listCrystallizers)
	m.HandleFunc("POST /api/v1/crystallizers/{id}/fill", h.fillCrystallizer)
	m.HandleFunc("POST /api/v1/crystallizers/{id}/transition", h.transitionCrystallizer)

	m.HandleFunc("POST /api/v1/harvests", h.openHarvest)
	m.HandleFunc("POST /api/v1/harvests/{id}/complete", h.completeHarvest)
	m.HandleFunc("GET /api/v1/harvests/monthly", h.monthlyHarvests)

	m.HandleFunc("POST /api/v1/pumps", h.createPump)
	m.HandleFunc("GET /api/v1/pumps", h.listPumps)
	m.HandleFunc("POST /api/v1/pumps/{id}/service", h.servicePump)

	m.HandleFunc("POST /api/v1/transfers", h.scheduleTransfer)
	m.HandleFunc("POST /api/v1/transfers/{id}/cancel", h.cancelTransfer)

	m.HandleFunc("POST /api/v1/weather", h.ingestWeather)
	m.HandleFunc("GET /api/v1/alerts", h.listAlerts)
	m.HandleFunc("POST /api/v1/alerts/{id}/ack", h.ackAlert)
	m.HandleFunc("POST /api/v1/alerts/{id}/close", h.closeAlert)

	m.HandleFunc("POST /api/v1/maintenance", h.planMaintenance)
	m.HandleFunc("POST /api/v1/maintenance/{id}/complete", h.completeMaintenance)

	m.HandleFunc("POST /api/v1/labs", h.recordLab)
	m.HandleFunc("GET /api/v1/labs", h.listLabs)
	m.HandleFunc("POST /api/v1/sensors", h.registerSensor)
	m.HandleFunc("POST /api/v1/sensors/{id}/calibrate", h.calibrateSensor)
	m.HandleFunc("GET /api/v1/pond-groups", h.pondGroups)
	m.HandleFunc("POST /api/v1/pond-groups", h.createGroup)
	m.HandleFunc("GET /api/v1/pond-groups/{id}/balance", h.groupBalance)

	m.HandleFunc("GET /api/v1/reports/daily.csv", h.dailyReport)
	m.HandleFunc("/", h.serveStatic)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	respond(w, http.StatusOK, map[string]string{"status": "ok"})
}

// serveStatic 静态页面（看板前端）。
func (h *Handler) serveStatic(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && !strings.HasPrefix(r.URL.Path, "/static/") {
		http.NotFound(w, r)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/static/")
	if name == "" || name == "/" {
		name = "index.html"
	}
	full := filepath.Join(h.staticDir, filepath.Clean("/"+name))
	data, err := os.ReadFile(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch {
	case strings.HasSuffix(name, ".html"):
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case strings.HasSuffix(name, ".js"):
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case strings.HasSuffix(name, ".css"):
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	_, _ = w.Write(data)
}
