{
  description = "My python flake";
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };
  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let pkgs = nixpkgs.legacyPackages.${system}; in
      {
        devShells.default =
          let
            changeVersion = overrideFunc: version: hash: overrideFunc (oldAttrs: rec {
              inherit version;
              src = oldAttrs.src.override {
                inherit version hash;
              };
            });

            pkgs-python = import nixpkgs {
              inherit system;
              config = {
                allowUnfree = true;
                packageOverrides = pkgs: rec{
                  # override package
                  # thrift = pkgs.thrift.overrideAttrs (old: { doCheck = false; });

                  
                };
              };
            };
      
            my-python = pkgs-python.python310.override {
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
            };
    });

}
