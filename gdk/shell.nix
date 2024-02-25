{ pkgs ? import /home/rxiao/nixpkgs {} }:
let
  gdk = pkgs.google-cloud-sdk.withExtraComponents( with pkgs.google-cloud-sdk.components; [
    gke-gcloud-auth-plugin
  ]);
in
  pkgs.mkShell {
    # nativeBuildInputs is usually what you want -- tools you need to run
    buildInputs =  [ gdk ];
}

