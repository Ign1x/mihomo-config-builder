{
  description = "mihomo-config-builder: deterministic mihomo config generator";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    let
      overlays.default = final: prev: {
        mihomo-config-builder = final.callPackage ./nix/package.nix { src = self; };
      };
    in
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [ overlays.default ];
        };
      in {
        packages.default = pkgs.mihomo-config-builder;
        packages.mihomo-config-builder = pkgs.mihomo-config-builder;

        apps.default = {
          type = "app";
          program = "${pkgs.mihomo-config-builder}/bin/mcb";
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gopls
            golangci-lint
            gotools
            go-tools
            just
          ];
        };
      }) // {
        overlays = overlays;

        nixosModules.default = import ./nix/module.nix;
        nixosModules.mihomo-config-builder = import ./nix/module.nix;
        homeManagerModules.default = import ./nix/home-manager.nix;
      };
}
