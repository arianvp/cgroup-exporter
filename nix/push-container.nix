{ writeShellApplication, skopeo, container }:
writeShellApplication {
  name = "push-container";
  runtimeInputs = [ skopeo ];
  text = ''
    ${container} | skopeo copy docker-archive:/dev/stdin "docker://''${1}@@unknown-digest@@" ''${2:+--additional-tag "$2"}
  '';
}
