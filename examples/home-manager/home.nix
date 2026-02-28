{ inputs, pkgs, ... }:
{
  imports = [
    inputs.mcb.homeManagerModules.default
  ];

  programs.mihomo-config-builder = {
    enable = true;
    package = inputs.mcb.packages.${pkgs.system}.default;
    profileFile = ./profile.yaml;
    outputFile = "${builtins.getEnv "HOME"}/.config/mihomo/config.yaml";
  };
}
