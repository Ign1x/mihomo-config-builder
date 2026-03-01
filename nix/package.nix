{ buildGoModule, lib, src ? ../. }:

buildGoModule {
  pname = "mihomo-config-builder";
  version = "0.2.1";
  inherit src;

  subPackages = [ "cmd/mcb" ];
  ldflags = [ "-s" "-w" ];

  vendorHash = "sha256-nRVeWwh5xVkXeg5um5AcIjzH9kTNZW8NL3JBTm0OjmM=";

  meta = with lib; {
    description = "Deterministic Mihomo configuration generator";
    homepage = "https://github.com/ign1x/mihomo-config-builder";
    license = licenses.mit;
    mainProgram = "mcb";
    platforms = platforms.unix;
  };
}
