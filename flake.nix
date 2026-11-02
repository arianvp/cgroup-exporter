{
  inputs.nixpkgs.url = "github:NixOS/nixpkgs?ref=nixpkgs-unstable";
  outputs = { self, nixpkgs }:
    let
      systems = [ "aarch64-linux" "x86_64-linux" ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
    in {
      nixosModules.default = { config, lib, pkgs, ... }: {
        imports = [ ./nix/module.nix ];

        services.prometheus.exporters.cgroup.package =
          lib.mkDefault self.packages.${pkgs.stdenv.hostPlatform.system}.default;
      };

      overlays.default = final: prev: {
        cgroup-exporter = final.callPackage ./nix/package.nix { };
      };

      packages = forAllSystems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          cgroup-exporter = pkgs.callPackage ./nix/package.nix { };
        in rec {
          default = cgroup-exporter;
          container = pkgs.callPackage ./nix/container.nix {
	    inherit cgroup-exporter;
          };
	  push-container = pkgs.callPackage ./nix/push-container.nix {
	    inherit container;
	  };
        });

      formatter = forAllSystems (system: nixpkgs.legacyPackages.${system}.nixfmt);

      devShells = forAllSystems (system: {
        default = with nixpkgs.legacyPackages.${system};
          mkShell {
            name = "cgroups-exporter";
            nativeBuildInputs = [ go ];
          };
      });

      checks = forAllSystems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          container = self.packages.${system}.container;
        in {
        package = self.packages.${system}.default;
        podman-kube-test = nixpkgs.lib.nixos.runTest {
          name = "cgroup-exporter-podman-kube";
          hostPkgs = pkgs;
          nodes.machine = {
            virtualisation.podman.enable = true;
          };
          testScript = ''
            machine.succeed("${container} | podman load")
            machine.succeed(
              "podman kube play --publish-all ${./deploy/daemonset.yaml}"
            )
            machine.wait_for_open_port(13232)
            machine.succeed("curl -sf http://localhost:13232/metrics | grep cgroup_memory_current_bytes")
          '';
        };
        integration-test = nixpkgs.lib.nixos.runTest {
          name = "cgroup-exporter";
          hostPkgs = nixpkgs.legacyPackages.${system};
          globalTimeout = 300;
          nodes.machine = {
            imports = [ self.nixosModules.default ];
            services.prometheus.exporters.cgroup.enable = true;
            services.prometheus.exporters.cgroup.port = 8080;
          };
          testScript = ''
            machine.wait_for_unit("cgroup-exporter.service", timeout=60)
            machine.wait_for_open_port(8080, timeout=30)
            metrics = machine.succeed("curl -sf http://localhost:8080/metrics")
            assert 'cgroup_memory_current_bytes{cgroup="system.slice"}' in metrics
            # system.slice has children, so this only works if inner cgroups
            # are collected as well.
            for metric in [
                "cgroup_cpu_pressure_waiting_seconds_total",
                "cgroup_io_pressure_waiting_seconds_total",
                "cgroup_memory_pressure_waiting_seconds_total",
            ]:
                assert f'{metric}{{cgroup="system.slice"}}' in metrics, (
                    f"no {metric} for system.slice"
                )
          '';
        };
      });
    };
}
