{ lib, buildGoModule }:

buildGoModule rec {
  pname = "cgroup-exporter";
  version = "0.3.2";

  src = lib.fileset.toSource {
    root = ../.;
    fileset = lib.fileset.unions [
      ../go.mod
      ../go.sum
      (lib.fileset.fileFilter (file: file.hasExt "go") ../.)
      ../collector
    ];
  };

  env.CGO_ENABLED = 0;

  vendorHash = "sha256-PzUdwc04criIThlCDoQKR9N3xBkRSc3UpEGwyBHIlYI=";

  passthru = { inherit version; };
}
