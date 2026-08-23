package handler

import (
	"net/http"

	"github.com/jb843051627/saltern-watch/internal/model"
)

type labReq struct {
	CrystallizerID int64   `json:"crystallizer_id"`
	NaMgRatio      float64 `json:"na_mg_ratio"`
	SulfatePPM     float64 `json:"sulfate_ppm"`
	InsolublePPM   float64 `json:"insoluble_ppm"`
	Analyst        string  `json:"analyst"`
}

func (h *Handler) recordLab(w http.ResponseWriter, r *http.Request) {
	var req labReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	l, err := h.Svc.Quality.RecordSample(req.CrystallizerID,
		req.NaMgRatio, req.SulfatePPM, req.InsolublePPM, req.Analyst)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusCreated, l)
}

func (h *Handler) listLabs(w http.ResponseWriter, r *http.Request) {
	crystID, err := intQuery(r, "crystallizer_id")
	if err != nil || crystID <= 0 {
		fail(w, model.ErrInvalidInput)
		return
	}
	samples, err := h.Svc.Store().ListLabSamplesByCryst(crystID, 50)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, samples)
}

type sensorReq struct {
	PondID int64  `json:"pond_id"`
	Kind   string `json:"kind"`
	Model  string `json:"model"`
}

func (h *Handler) registerSensor(w http.ResponseWriter, r *http.Request) {
	var req sensorReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	sen, err := h.Svc.Sensors.Register(req.PondID, req.Kind, req.Model)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusCreated, sen)
}

type calibrateReq struct {
	ReferenceBe float64 `json:"reference_be"`
	RawBe       float64 `json:"raw_be"`
}

func (h *Handler) calibrateSensor(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, err)
		return
	}
	var req calibrateReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	rec, err := h.Svc.Sensors.Calibrate(id, req.ReferenceBe, req.RawBe)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, rec)
}

func (h *Handler) pondGroups(w http.ResponseWriter, _ *http.Request) {
	groups, err := h.Svc.PondGroups.List()
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, groups)
}

type createGroupReq struct {
	Name      string  `json:"name"`
	PondIDs   []int64 `json:"pond_ids"`
	MinKeepCm float64 `json:"min_keep_cm"`
}

func (h *Handler) createGroup(w http.ResponseWriter, r *http.Request) {
	var req createGroupReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	g, err := h.Svc.PondGroups.Create(req.Name, req.PondIDs, req.MinKeepCm)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusCreated, g)
}

func (h *Handler) groupBalance(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, err)
		return
	}
	plan, err := h.Svc.PondGroups.BalancePlan(id)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, plan)
}
