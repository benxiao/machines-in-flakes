{
  description = "Ultralytics";
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };
  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let pkgs = import nixpkgs {
        inherit system;
        config.allowUnfree = true; 
      }; in
      {
        devShells.default = (pkgs.buildFHSUserEnv {
          name = "pipzone";
          targetPkgs = pkgs: (with pkgs; [
            python3
            python3.pkgs.pip
            poetry
            python3.pkgs.virtualenv
            gcc
            pkg-config
            cairo.dev
            # for opencv
            xorg.libXext
            xorg.libSM
            xorg.libxcb.dev
            xorg.libICE
            xorg.libX11.dev
            xorg.xorgproto
            glib.dev
            gobject-introspection.dev
            libffi.dev
            cmake
            zlib
            libglvnd
          ]);
          runScript = "bash";
        }).env;
    });
}
