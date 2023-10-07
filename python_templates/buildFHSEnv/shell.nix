{ pkgs ? import /home/rxiao/nixpkgs {
    config = {
      allowUnfree = true;
      # cudaSupport = true;
      # cudaCapabilities = [ "8.6" ];
    };
  }
}:
  
(pkgs.buildFHSUserEnv {
  name = "python3-env";
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
  }).env
