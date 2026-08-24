package regression

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jb843051627/saltern-watch/internal/clock"
	"github.com/jb843051627/saltern-watch/internal/config"
	"github.com/jb843051627/saltern-watch/internal/handler"
	"github.com/jb843051627/saltern-watch/internal/service"
	"github.com/jb843051627/saltern-watch/internal/store"
)

func TestBug16_StageAdvanceRunawayGuards(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Unix(1750000000, 0)
	svc := service.New(st, clock.NewFake(now), &config.Config{LocalZone: time.UTC, MaxBatchReads: 10})

	final, err := svc.Ponds.Create("E5", 1000, 4, 26, 80)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Ponds.AdvanceStage(final.ID, 27); err == nil {
		t.Fatal("stage 4 pond must not advance further")
	}
	got, _ := svc.Ponds.Get(final.ID)
	if got.Stage != 4 || got.TargetBe != 26 {
		t.Fatalf("final pond mutated: stage=%d target=%.1f", got.Stage, got.TargetBe)
	}

	fresh, err := svc.Ponds.Create("E6", 1000, 0, 7, 100)
	if err != nil {
		t.Fatal(err)
	}
	dash := service.NewDashboard(st, clock.NewFake(now), svc.Reports)
	ts := httptest.NewServer(handler.New(svc, dash, dir))
	defer ts.Close()
	resp, err := http.Post(ts.URL+"/api/v1/ponds/"+itoa16(fresh.ID)+"/advance", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("advance without readings status = %d, want 404", resp.StatusCode)
	}
}

func itoa16(v int64) string {
	if v == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}
