package main

import (
"context"
"net/http"
"os"
"os/signal"
"syscall"

hclog "github.com/hashicorp/go-hclog"
nomadapi "github.com/hashicorp/nomad/api"

observer "github.com/gulducat/autoscaler-holodeck/observer"
)

func main() {
logger := hclog.New(&hclog.LoggerOptions{
Name:  "observer",
Level: hclog.LevelFromString(os.Getenv("LOG_LEVEL")),
})

if os.Getenv("NOMAD_ADDR") == "" {
logger.Error("NOMAD_ADDR must be set")
os.Exit(1)
}

cfg := nomadapi.DefaultConfig()
if cfg.SecretID == "" {
logger.Error("NOMAD_TOKEN must be set")
os.Exit(1)
}
if cfg.Namespace == "" {
cfg.Namespace = "default"
}

nomadClient, err := nomadapi.NewClient(cfg)
if err != nil {
logger.Error("failed to create Nomad client", "error", err)
os.Exit(1)
}

addr := os.Getenv("OBSERVER_ADDR")
if addr == "" {
addr = ":9090"
}

store := observer.NewStore()
listener := observer.NewStreamListener(nomadClient, store, logger.Named("stream"))
srv := observer.NewServer(store)

ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer stop()

go func() {
if err := listener.Run(ctx); err != nil && err != context.Canceled {
logger.Error("stream listener stopped", "error", err)
}
}()

httpSrv := &http.Server{Addr: addr, Handler: srv.Handler()}
go func() {
<-ctx.Done()
httpSrv.Shutdown(context.Background()) //nolint:errcheck
}()

logger.Info("starting observer", "addr", addr)
if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
logger.Error("server error", "error", err)
os.Exit(1)
}
}
