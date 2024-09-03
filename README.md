# Control Group V2 exporter

This is a lightweight Prometheus exporter for cgroups that only supports
the unified cgroup v2 hierarchy. It exposes usage metrics for each cgroup
in the hierarchy.

Metrics supported are:

* Pressure stall information (`io.pressure`, `memory.pressure`, `cpu.pressure`). Useful as a leading indicator for performance issues.
* Events (like OOM, hitting max CPU, Memory, IO, etc) (`io.events`, `memory.events`)
* Resource usage (`memory.usage`, `cpu.usage`) and limits (`io.max`, `memory.{min,low,high,max}`, `cpu.{min,low,high,max}`)
* Detailed resource usage (`io.stat`, `memory.stat`, `cpu.stat`)
    - `io.stat` gives IOPS and bytes read/written per device
    - `memory.stat` gives page faults, cache, swap, etc
    - `cpu.stat` gives number of times the CPU was throttled, time spent in different states, etc


Systemd dropped support for the legacy cgroup hierarchy in version 256.
So there is no point in having the complexity of supporting both cgroup
versions.


## Why another exporter?

Cgroup exposes a lot of metrics. This can quickly become overwhelming. Non
of the other solutions allow you to enable and disable certain metrics to be
collected throughout the hierarchy. Neither does this exporter, but we have
an issue open for it: https://github.com/arianvp/cgroup-exporter/issues/1

[`google/cadvisor`](https://github.com/google/cadvisor) is too heavy-weight, tries to
do way more than cgroups, tries to support both cgroupv1 and cgroupv2, and is
missing a lot of metrics (like pressure stall information). Furthermore they
focus on "containers", whilst cgroups and containers are not synonyms. Cgroups
are used for all services on a modern linux system for resource management; not just containers.

[`mosquito/cgroups-exporter`](https://github.com/mosquito/cgroups-exporter) also comes with the
baggage of supporting both cgroupv1 and cgroupv2, and is missing a lot of metrics.


[`treydock/cgroup_exporter`](https://github.com/treydock/cgroup_exporter) also comes with the
baggage of supporting both cgroupv1 and cgroupv2, and is missing a lot of metrics.

