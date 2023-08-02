{ pkgs ? import <nixpkgs> {
    config = {
      allowUnfree = true;
      packageOverrides = pkgs: rec{
        # override package
        # thrift = pkgs.thrift.overrideAttrs (old: { doCheck = false; });

      };
    };
}}:
let
  changeVersion = overrideFunc: version: hash: overrideFunc (oldAttrs: rec {
    inherit version;
    src = oldAttrs.src.override {
      inherit version hash;
    };
  });

  my-python = pkgs.python310.override {
    # super important
    self = my-python;
    packageOverrides = self: super: {
      mmengine = super.mmengine.overridePythonAttrs(old: { doCheck = false; nativeBuildInputs = []; });
      # override python packages
      # torch = super.torch-bin;
      # torchvision = super.torchvision-bin;
      # numpy = changeVersion super.numpy.overridePythonAttrs "1.23.5" "sha256-Gxdm1vOXwYFT1AAV3fx53bcVyrrcBNLSKNTlqLxN7Ro=";
    };
  };


in 
  pkgs.mkShell {
    buildInputs = [
      pkgs.just
      (my-python.withPackages (p: with p; [
        # pandas
        # cramjam
        mmengine
        pytest
      ]))
    ];
  }
  

