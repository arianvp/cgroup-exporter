{
  inputs.nixpkgs.url = "github:NixOS/nixpkgs?ref=nixpkgs-unstable";
  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "aarch64-linux"
        "x86_64-linux"
      ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
    in
    {
      nixosModules.default =
        {
          lib,
          pkgs,
          ...
        }:
        {
          imports = [ ./nix/module.nix ];
          services.prometheus.exporters.cgroup.package = lib.mkDefault self.packages.${pkgs.system}.default;
        };

      overlays.default = final: prev: {
        cgroup-exporter = final.callPackage ./nix/package.nix { };
      };

      packages = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          cgroup-exporter = pkgs.callPackage ./nix/package.nix { };
        in
        rec {
          default = cgroup-exporter;
          container = pkgs.callPackage ./nix/container.nix {
            inherit cgroup-exporter;
          };
          push-container = pkgs.callPackage ./nix/push-container.nix {
            inherit container;
          };
          github-eval-jobs = pkgs.callPackage ./nix/github-eval-jobs.nix { flake = self; };
        }
      );

      devShells = forAllSystems (system: {
        default =
          with nixpkgs.legacyPackages.${system};
          mkShell {
            name = "cgroups-exporter";
            nativeBuildInputs = [ go ];
          };
      });

      checks = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          packages = self.packages.${system};

          inherit (pkgs) testers;
        in
        {
          actionlint = pkgs.runCommand "actionlint" { nativeBuildInputs = [ pkgs.actionlint ]; } ''
            actionlint ${./.github/workflows/ci.yml}
            touch $out
          '';

          zizmor = pkgs.runCommand "actionlint" { nativeBuildInputs = [ pkgs.zizmor ]; } ''
            zizmor ${./.github}
            touch $out
          '';

          # testers.runCommand is a neat little hack that allows you to access network in nix builds
          # https://nixos.org/manual/nixpkgs/unstable/#tester-runCommand
          push-container = testers.runCommand {
            name = "push-container";
            nativeBuildInputs = [
              pkgs.cacert
              pkgs.skopeo
            ];
            impureEnvVars = [
              "GITHUB_ACTOR"
              "GITHUB_TOKEN"
              "GITHUB_REPOSITORY"
              "GITHUB_REF"
              "GITHUB_EVENT_NAME"
            ];
            meta.permissions = {
              packages = "write";
              attestations = "write";
              id-token = "write";
            };
            script = ''
              skopeo login ghcr.io --username "$GITHUB_ACTOR" --password "$GITHUB_TOKEN"
              [[ $GITHUB_EVENT_NAME = release ]] && TAG=''${GITHUB_REF#refs/tags/}
              ${packages.container} | \
                skopeo copy \
                  docker-archive:/dev/stdin \
                  "docker://ghcr.io/''${GITHUB_REPOSITORY}@@unknown-digest@@" \
                  ''${tag:+--additional-tag $tag}
              touch $out
            '';
          };

          actions-test = testers.runNixOSTest {
            # NOTE: The whole docker dance here is kinda pointless as this is running in an isolated NixOS VM already
            # but act insists on docker. I wish it could run in "dockerless" mode. Or perhaps we should write our own
            # mock actions runner.  In the end, we want to assert that we're not relying on third party actions and
            # only relying on nix in our pipelines, after all.
            name = "actions-test";
            nodes.machine = {
              virtualisation.podman = {
                enable = true;
                dockerSocket.enable = true;
                # TODO: Preload nix image
              };
              environment.systemPackages = [ pkgs.act ];
              environment.etc.repo.source = ./.;
            };
            testScript = ''
              machine.succeed("cd /repo && act -P ubuntu-latest=ghcr.io/nixos/nix:2.33.1  --action-offline-mode")
            '';
          };

          integration-test = testers.runNixOSTest {
            name = "cgroup-exporter";
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

          container-integration-test = testers.runNixOSTest {
            name = "cgroup-exporter-container";
            nodes.machine = {
              virtualisation.podman.enable = true;
              virtualisation.oci-containers.containers.cgroup-exporter = {
                image = "cgroup-exporter";
                imageStream = self.packages.${system}.container;
                volumes = [ "/sys/fs/cgroup:/sys/fs/cgroup" ];
              };
            };
            testScript = ''
              machine.wait_for_unit("multi-user.target")
              # TODO: Test if container works
            '';
          };
        }
      );
    };
}
