{
  inputs,
  pkgs,
  ...
}:
{
  imports = [
    inputs.mcb.nixosModules.default
  ];

  services.mihomo-config-builder = {
    enable = true;
    package = inputs.mcb.packages.${pkgs.system}.default;
    profileFile = /etc/mcb/profile.yaml;
    outputFile = "/var/lib/mihomo/config.yaml";
    schedule = "*:0/30";
    mihomoServiceName = "mihomo.service";
  };
}
