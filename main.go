package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/arianvp/cgroup-exporter/collector"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	opts := make([]collector.Option, 0, len(collector.CollectorStatOptions)+2)
	addr := flag.String("listen-address", ":13232", "address to listen on")
	cgroup := flag.String("cgroup", "*", "what cgroup to monitor. Can be a blob. If empty all cgroups are monitored.")
	root := flag.String("cgroup.dir", collector.LinuxCGroupsDir, "Filesystem root where the Kernel exposes its CGroup information")

	for c, o := range collector.CollectorStatOptions {
		fn := func(v string) error {
			if f, err := strconv.ParseBool(v); err != nil {
				return err
			} else if f {
				opts = append(opts, o)
			}

			return nil
		}

		flag.BoolFunc("collector."+c, "enable the "+c+" collector", fn)
	}

	flag.Parse()

	if len(opts) == 0 {
		// add all collectors if none are selected
		for _, o := range collector.CollectorStatOptions {
			opts = append(opts, o)
		}
	}

	opts = append(opts, collector.WithCGroupGlob(*cgroup), collector.WithCGroupFS(os.DirFS(*root)))
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector.NewCollector(opts...))
	http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{Registry: registry}))
	// TODO: cancellation
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal(err)
	}
}
