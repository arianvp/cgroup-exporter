{
  inputs.nixpkgs.url = "github:NixOS/nixpkgs?ref=nixpkgs-unstable";
  outputs = { self, nixpkgs }: {
    nixosModules.default = { config, lib, ... }:
      let cfg = config.services.prometheus.exporters.cgroup; in {
        options.services.prometheus.exporters.cgroup = {
          enable = lib.mkEnableOption "cgroup-exporter";
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
              ExecStart = "${self.packages.x86_64-linux.default}/bin/cgroup-exporter -listen-address ${cfg.listenAddress}:${toString cfg.port}";
              Restart = "always";
            };
          };
        };
      };


    packages.x86_64-linux.default = nixpkgs.legacyPackages.x86_64-linux.buildGoModule {
      pname = "cgroup-exporter";
      version = "0.1.0";
      src = ./.;
      vendorHash = "sha256-srQcHjMVz9wV96eAX9P9iRtvi7CHqZC+GZSsh+gkrvU=";
    };

    devShells.x86_64-linux.default = with nixpkgs.legacyPackages.x86_64-linux; mkShell {
      name = "cgroups-exporter";
      nativeBuildInputs = [ go ];
    };

    checks.x86_64-linux.integration-test = nixpkgs.lib.nixos.runTest {
      name = "cgroup-exporter";
      hostPkgs = nixpkgs.legacyPackages.x86_64-linux;
      nodes.machine = {
        imports = [ self.nixosModules.default ];
        services.prometheus.exporters.cgroup.enable = true;
        services.prometheus.exporters.cgroup.port = 8080;
      };
      testScript = ''
        machine.wait_for_unit("cgroup-exporter.service");
        machine.succeed("curl http://localhost:8080/metrics");
      '';
    };

  };
}
