{ buildGoModule, lib, src ? ../. }:

buildGoModule {
  pname = "mihomo-config-builder";
  version = "0.3.1";
  inherit src;

  subPackages = [ "cmd/mcb" ];
  ldflags = [ "-s" "-w" ];

  # Dependencies are reviewed and shipped in vendor/. No module download
  # derivation or runtime package manager is needed to build or run mcb.
  vendorHash = null;
  env.CGO_ENABLED = "0";
  env.GOPROXY = "off";
  env.GOSUMDB = "off";

  meta = with lib; {
    description = "Deterministic Mihomo configuration generator";
    homepage = "https://github.com/ign1x/mihomo-config-builder";
    license = licenses.mit;
    mainProgram = "mcb";
    platforms = platforms.unix;
  };
}
