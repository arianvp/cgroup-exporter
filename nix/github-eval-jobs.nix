{ writeShellApplication, jq , nix-eval-jobs, flake }: writeShellApplication {
  name = "github-eval-jobs";
  runtimeInputs = [ jq nix-eval-jobs ];
  runtimeEnv.flake = flake;
  text = ''
    nix-eval-jobs --flake "path:$flake" --meta --select 'flake: {inherit (flake.outputs) checks packages;}'  --force-recurse --workers 4 | jq --slurp 
  '';
}
