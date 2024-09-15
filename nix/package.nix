{ buildGoModule }:

buildGoModule {
  pname = "cgroup-exporter";
  version = "0.1.0";
  src = ../.;
  vendorHash = "sha256-srQcHjMVz9wV96eAX9P9iRtvi7CHqZC+GZSsh+gkrvU=";
}
