{ config, lib, pkgs, ... }:
let cfg = config.services.prometheus.exporters.cgroup; in {
  options.services.prometheus.exporters.cgroup = {
    enable = lib.mkEnableOption "cgroup-exporter";
    package = lib.mkPackageOption pkgs "cgroup-exporter" { };
    listenAddress = lib.mkOption {
      type = lib.types.str;
      default = "[::]";
      description = "Address to listen on";
    };
    port = lib.mkOption {
      type = lib.types.int;
      default = 13232;
      description = "Port to listen on";
    };
  };
  config = lib.mkIf cfg.enable {
    systemd.services.cgroup-exporter = {
      description = "cgroup-exporter";
      wantedBy = [ "multi-user.target" ];
      serviceConfig = {
        Type = "simple";
        ExecStart = "${cfg.package}/bin/cgroup-exporter -listen-address ${cfg.listenAddress}:${toString cfg.port}";
        Restart = "always";
      };
    };
  };
}
