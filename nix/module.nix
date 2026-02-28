{ config, lib, pkgs, ... }:

let
  cfg = config.services.mihomo-config-builder;
in {
  options.services.mihomo-config-builder = {
    enable = lib.mkEnableOption "Generate mihomo config from profile";

    package = lib.mkOption {
      type = lib.types.package;
      default = pkgs.callPackage ./package.nix { };
      description = "mcb package to run";
    };

    profileFile = lib.mkOption {
      type = lib.types.path;
      example = "/etc/mcb/profile.yaml";
      description = "Path to profile.yaml";
    };

    outputFile = lib.mkOption {
      type = lib.types.str;
      default = "/var/lib/mihomo/config.yaml";
      description = "Output path for generated config";
    };

    schedule = lib.mkOption {
      type = lib.types.str;
      default = "*:0/30";
      description = "systemd timer OnCalendar value";
    };

    mihomoServiceName = lib.mkOption {
      type = lib.types.str;
      default = "mihomo.service";
      description = "Systemd service name to restart after successful generation";
    };
  };

  config = lib.mkIf cfg.enable {
    systemd.services.mihomo-config-builder = {
      description = "Build mihomo config via mcb";
      serviceConfig = {
        Type = "oneshot";
        ExecStart = "${cfg.package}/bin/mcb build -c ${cfg.profileFile} -o ${cfg.outputFile}";
        ExecStartPost = "${pkgs.systemd}/bin/systemctl try-restart ${cfg.mihomoServiceName}";
      };
      wantedBy = [ "multi-user.target" ];
    };

    systemd.timers.mihomo-config-builder = {
      description = "Periodic mihomo config regeneration";
      wantedBy = [ "timers.target" ];
      timerConfig = {
        OnCalendar = cfg.schedule;
        Persistent = true;
        Unit = "mihomo-config-builder.service";
      };
    };
  };
}
