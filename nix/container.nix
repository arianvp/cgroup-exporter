{ dockerTools, cgroup-exporter }:
dockerTools.streamLayeredImage {
  name = "cgroup-exporter";
  config = {
    Entrypoint = [ "${cgroup-exporter}/bin/cgroup-exporter" ];
    Cmd = [
      "-listen-address"
      ":13232"
    ];
    ExposedPorts."13232/tcp" = { };
    User = "65534:65534"; # nobody:nobody
    Volumes."/sys/fs/cgroup" = { };
  };
}
