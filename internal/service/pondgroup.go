package service

import (
	"fmt"
	"sort"

	"github.com/jb843051627/saltern-watch/internal/clock"
	"github.com/jb843051627/saltern-watch/internal/model"
	"github.com/jb843051627/saltern-watch/internal/store"
)

// PondGroup 蒸发池串联组（梯度推进链路）。
type PondGroup struct {
	ID       int64
	Name     string
	PondIDs  []int64 // 按海水流向排序
	MinKeepCm float64
}

// PondGroupService 池组业务：梯度校验、组内均衡、链式输卤建议。
type PondGroupService struct {
	st    *store.DB
	clock clock.Clock
}

// Create 建池组并校验梯度（stage 必须严格递增）。
func (s *PondGroupService) Create(name string, pondIDs []int64, minKeepCm float64) (*PondGroup, error) {
	if len(pondIDs) < 2 {
		return nil, fmt.Errorf("%w: group needs at least 2 ponds", model.ErrInvalidInput)
	}
	var lastStage = -1
	for _, id := range pondIDs {
		p, err := s.st.GetPond(id)
		if err != nil {
			return nil, fmt.Errorf("pond %d: %w", id, err)
		}
		if p.Stage <= lastStage {
			return nil, fmt.Errorf("%w: pond %d stage %d breaks ascending gradient",
				model.ErrInvalidInput, id, p.Stage)
		}
		lastStage = p.Stage
	}
	res, err := s.st.SQL().Exec(
		`INSERT INTO pond_groups(name,pond_ids,min_keep_cm) VALUES(?,?,?)`,
		name, joinIDs(pondIDs), minKeepCm)
	if err != nil {
		return nil, fmt.Errorf("store: insert pond group: %w", err)
	}
	id, _ := res.LastInsertId()
	return &PondGroup{ID: id, Name: name, PondIDs: pondIDs, MinKeepCm: minKeepCm}, nil
}

// List 全部池组。
func (s *PondGroupService) List() ([]*PondGroup, error) {
	rows, err := s.st.SQL().Query(`SELECT id,name,pond_ids,min_keep_cm FROM pond_groups ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("store: list groups: %w", err)
	}
	defer rows.Close()
	out := make([]*PondGroup, 0, 4)
	for rows.Next() {
		var g PondGroup
		var ids string
		if err := rows.Scan(&g.ID, &g.Name, &ids, &g.MinKeepCm); err != nil {
			return nil, fmt.Errorf("store: scan group: %w", err)
		}
		g.PondIDs = splitIDs(ids)
		out = append(out, &g)
	}
	return out, rows.Err()
}

// GradientGaps 找出组内浓度倒挂的相邻对（上游 Bé ≥ 下游 Bé 视为正常，
// 反之说明串池或误操作），返回问题对。
func (s *PondGroupService) GradientGaps(groupID int64) ([][2]int64, error) {
	groups, err := s.List()
	if err != nil {
		return nil, err
	}
	var target *PondGroup
	for _, g := range groups {
		if g.ID == groupID {
			target = g
			break
		}
	}
	if target == nil {
		return nil, model.ErrNotFound
	}
	beSvc := &BrineService{st: s.st, clock: s.clock}
	var bad [][2]int64
	for i := 0; i+1 < len(target.PondIDs); i++ {
		up := target.PondIDs[i]
		down := target.PondIDs[i+1]
		upBe, err1 := beSvc.CurrentBe(up)
		downBe, err2 := beSvc.CurrentBe(down)
		if err1 != nil || err2 != nil {
			continue
		}
		if upBe < downBe-0.5 {
			bad = append(bad, [2]int64{up, down})
		}
	}
	return bad, nil
}

// BalancePlan 组内液位均衡计划：高液位池 → 低液位池的建议转移量。
func (s *PondGroupService) BalancePlan(groupID int64) ([]BalanceMove, error) {
	groups, err := s.List()
	if err != nil {
		return nil, err
	}
	var target *PondGroup
	for _, g := range groups {
		if g.ID == groupID {
			target = g
			break
		}
	}
	if target == nil {
		return nil, model.ErrNotFound
	}
	type levelRow struct {
		id    int64
		level float64
		area  float64
	}
	rows := make([]levelRow, 0, len(target.PondIDs))
	for _, id := range target.PondIDs {
		p, err := s.st.GetPond(id)
		if err != nil {
			return nil, err
		}
		rows = append(rows, levelRow{id: p.ID, level: p.BrineLevelCm, area: p.AreaM2})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].level > rows[j].level })
	var plan []BalanceMove
	for len(rows) >= 2 {
		src := rows[0]
		dst := rows[len(rows)-1]
		avg := (src.level*src.area + dst.level*dst.area) / (src.area + dst.area)
		moveCm := avg - dst.level
		if moveCm < 5 {
			break
		}
		vol := moveCm * dst.area / 100.0
		if src.level-moveCm < target.MinKeepCm {
			break
		}
		plan = append(plan, BalanceMove{FromPondID: src.id, ToPondID: dst.id, VolumeM3: vol})
		rows[0].level -= vol * 100 / src.area
		rows[len(rows)-1].level += vol * 100 / dst.area
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].level > rows[j].level })
	}
	return plan, nil
}

// BalanceMove 单条均衡建议。
type BalanceMove struct {
	FromPondID int64   `json:"from_pond_id"`
	ToPondID   int64   `json:"to_pond_id"`
	VolumeM3   float64 `json:"volume_m3"`
}

func joinIDs(ids []int64) string {
	out := ""
	for i, v := range ids {
		if i > 0 {
			out += ","
		}
		out += fmt.Sprintf("%d", v)
	}
	return out
}

func splitIDs(s string) []int64 {
	var out []int64
	cur := int64(0)
	has := false
	for _, c := range s + "," {
		if c >= '0' && c <= '9' {
			cur = cur*10 + int64(c-'0')
			has = true
		} else if has {
			out = append(out, cur)
			cur = 0
			has = false
		}
	}
	return out
}
