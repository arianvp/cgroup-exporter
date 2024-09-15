{
  inputs.nixpkgs.url = "github:NixOS/nixpkgs?ref=nixpkgs-unstable";
  outputs = { self, nixpkgs }: {
    nixosModules.default = { config, lib, ... }: {
      imports = [
        ./nix/module.nix
      ];

      services.prometheus.exporters.cgroup.package = lib.mkDefault self.packages.x86_64-linux.default;
    };

    overlays.default = final: prev: {
      cgroup-exporter = final.callPackage ./nix/package.nix { };
    };

    packages.x86_64-linux.default = import ./nix/package.nix {
      inherit (nixpkgs.legacyPackages.x86_64-linux) buildGoModule;
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
