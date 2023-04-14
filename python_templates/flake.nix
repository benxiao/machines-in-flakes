{
  description = "Highlighter All";
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
              # tensorflow-2.11 is marked as insecure why are we support tensorflow?
              config = {
                allowUnfree = true;
                packageOverrides = pkgs: rec{
                  thrift = pkgs.thrift.overrideAttrs (old: { doCheck = false; });
                  python3 = pkgs.python3.override {
                    # super important
                    self = python3;
                    packageOverrides = self: super: {
                      torch = super.torch-bin;
                      torchvision = super.torchvision-bin;
                      numpy = changeVersion super.numpy.overridePythonAttrs "1.23.5" "sha256-Gxdm1vOXwYFT1AAV3fx53bcVyrrcBNLSKNTlqLxN7Ro=";
                    };
                  };
                };
              };
            };
          in
            pkgs.mkShell {
              buildInputs = [
                pkgs.just
                (pkgs-python.python3.withPackages (p: with p; [
                  pandas
                  cramjam
                ]))
              ];
            };
    });

}
