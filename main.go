package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/arianvp/cgroup-exporter/collector"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	addr := flag.String("listen-address", ":13232", "address to listen on")
	cgroup := flag.String("cgroup", "", "what cgroup to monitor. Can be a blob. If empty all cgroups are monitored.")
	flag.Parse()
	cgroupfs := os.DirFS("/sys/fs/cgroup")
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector.New(cgroupfs, *cgroup))
	http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{Registry: registry}))
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	ctx, cancelCause := context.WithCancelCause(ctx)
	go func() {
		cancelCause(http.ListenAndServe(*addr, nil))
	}()
	<-ctx.Done()
	if err := context.Cause(ctx); err != nil &&
		!errors.Is(err, context.Canceled) &&
		!errors.Is(err, context.DeadlineExceeded) &&
		errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
