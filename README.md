# Control Group V2 exporter

This is a lightweight Prometheus exporter for cgroups that only supports
the unified cgroup v2 hierarchy.

Systemd dropped support for the legacy cgroup hierarchy in version 256.
So there is no point in having the complexity of supporting both cgroup
versions.