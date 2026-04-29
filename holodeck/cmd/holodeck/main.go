package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	hclog "github.com/hashicorp/go-hclog"
	nomadapi "github.com/hashicorp/nomad/api"

	holodeck "github.com/gulducat/autoscaler-holodeck/holodeck"
)

func main() {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "holodeck",
		Level: hclog.LevelFromString(os.Getenv("LOG_LEVEL")),
	})

	addr := os.Getenv("HOLODECK_ADDR")
	if addr == "" {
		addr = ":9091"
	}

	observerAddr := os.Getenv("OBSERVER_ADDR")
	if observerAddr == "" {
		logger.Warn("OBSERVER_ADDR not set; Observer reporting disabled")
	}

	tracker := holodeck.NewNomadTracker()
	observerClient := holodeck.NewObserverClient(observerAddr, logger.Named("observer"))
	manager := holodeck.NewWorldManager(tracker, observerClient)
	srv := holodeck.NewServer(manager)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start Nomad poller only if NOMAD_ADDR is configured.
	nomadAddr := os.Getenv("NOMAD_ADDR")
	if nomadAddr == "" {
		logger.Warn("NOMAD_ADDR not set; capacity-coupled metrics will use alloc/node count of 0")
	} else {
		cfg := nomadapi.DefaultConfig()
		if cfg.Namespace == "" {
			cfg.Namespace = "default"
		}
		nomadClient, err := nomadapi.NewClient(cfg)
		if err != nil {
			logger.Error("failed to create Nomad client", "error", err)
			os.Exit(1)
		}
		poller := holodeck.NewNomadPoller(nomadClient, tracker, logger.Named("nomad"))
		go poller.Run(ctx)
	}

	httpSrv := &http.Server{Addr: addr, Handler: srv.Handler()}
	go func() {
		<-ctx.Done()
		httpSrv.Shutdown(context.Background()) //nolint:errcheck
	}()

	logger.Info("starting holodeck", "addr", addr)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}
