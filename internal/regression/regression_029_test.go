package regression

import (
	"bytes"
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

func TestBug29_SensorOffsetAppliedInIngest(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Unix(1750000000, 0)
	ck := clock.NewFake(now)
	cfg := &config.Config{LocalZone: time.UTC, MaxBatchReads: 10}
	svc := service.New(st, ck, cfg)
	dash := service.NewDashboard(st, ck, svc.Reports)

	pond, err := svc.Ponds.Create("E1", 1000, 0, 7, 100)
	if err != nil {
		t.Fatal(err)
	}
	sen, err := svc.Sensors.Register(pond.ID, "be", "BE-100")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Sensors.Calibrate(sen.ID, 11, 10); err != nil {
		t.Fatal(err)
	}

	body := `{"be":10,"temp_c":20,"level_cm":99,"source":"sensor:` + itoa29(sen.ID) + `"}`
	ts := httptest.NewServer(handler.New(svc, dash, dir))
	defer ts.Close()
	resp, err := http.Post(ts.URL+"/api/v1/ponds/"+itoa29(pond.ID)+"/readings", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	cur, err := svc.Brine.CurrentBe(pond.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cur != 11 {
		t.Fatalf("calibrated current be = %.3f, want 11 (sensor offset applied)", cur)
	}
	rd, err := svc.Brine.Latest(pond.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(rd.Source, "sensor:"+itoa29(sen.ID)) {
		t.Fatalf("source mangled in HTTP ingest: %q", rd.Source)
	}
	var buf bytes.Buffer
	buf.WriteString("")
	_ = buf
}

func itoa29(v int64) string {
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
