{ config, lib, pkgs, ... }:

let
  cfg = config.programs.mihomo-config-builder;
in {
  options.programs.mihomo-config-builder = {
    enable = lib.mkEnableOption "Run mcb from Home Manager";

    package = lib.mkOption {
      type = lib.types.package;
      default = pkgs.callPackage ./package.nix { };
      description = "mcb package";
    };

    profileFile = lib.mkOption {
      type = lib.types.path;
      description = "Path to profile.yaml";
    };

    outputFile = lib.mkOption {
      type = lib.types.str;
      default = "${config.home.homeDirectory}/.config/mihomo/config.yaml";
      description = "Generated config destination";
    };
  };

  config = lib.mkIf cfg.enable {
    home.packages = [ cfg.package ];

    systemd.user.services.mihomo-config-builder = {
      Unit = {
        Description = "Build mihomo config via mcb";
      };
      Service = {
        Type = "oneshot";
        ExecStart = "${cfg.package}/bin/mcb build -c ${cfg.profileFile} -o ${cfg.outputFile}";
      };
      Install = {
        WantedBy = [ "default.target" ];
      };
    };
  };
}
