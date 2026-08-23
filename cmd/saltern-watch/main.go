// saltern-watch 盐田制盐生产监控调度服务。
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jb843051627/saltern-watch/internal/clock"
	"github.com/jb843051627/saltern-watch/internal/config"
	"github.com/jb843051627/saltern-watch/internal/engine"
	"github.com/jb843051627/saltern-watch/internal/handler"
	"github.com/jb843051627/saltern-watch/internal/service"
	"github.com/jb843051627/saltern-watch/internal/store"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("[saltern] ")

	cfg, err := config.FromEnv()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	st, err := store.Open(cfg.DSN())
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	ck := clock.Real()
	svc := service.New(st, ck, cfg)
	dash := service.NewDashboard(st, ck, svc.Reports)

	if seed := os.Getenv("SALTERN_SEED_DEMO"); seed == "1" {
		if err := seedDemo(svc, cfg.LocalZone); err != nil {
			log.Printf("seed demo data: %v", err)
		}
	}

	h := handler.New(svc, dash, cfg.StaticDir)
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		eng := engine.New(svc, cfg, ck)
		if err := eng.Run(ctx); err != nil {
			log.Printf("engine exited: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("listening on %s (db=%s)", cfg.HTTPAddr, cfg.DBPath)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("http server: %v", err)
	}
	log.Printf("server stopped")
}

// seedDemo 空库时写入演示数据，便于本地启动后直接查看看板。
func seedDemo(svc *service.Service, loc *time.Location) error {
	ponds, err := svc.Ponds.List()
	if err != nil || len(ponds) > 0 {
		return err
	}
	now := time.Now().In(loc)
	if _, err := svc.Ponds.Create("E1-进水", 2400, 0, 7.0, 120); err != nil {
		return err
	}
	if _, err := svc.Ponds.Create("E2-一级", 1800, 1, 11.5, 90); err != nil {
		return err
	}
	if _, err := svc.Crystallizers.Create("C1", 60); err != nil {
		return err
	}
	if _, err := svc.Pumps.Create("P-A", 50); err != nil {
		return err
	}
	_, err = svc.Weather.Ingest(28.5, 0.62, 4.2, 0, now)
	return err
}
