{
  description = "protoc-gen-go-defaults - A protoc plugin that generates SetDefaults() methods from proto extensions";

  nixConfig = {
    extra-substituters = [ "https://ralch.cachix.org" ];
    extra-trusted-public-keys = [ "ralch.cachix.org-1:ti5RSqCJwd+VIv7dlxJEAVD98KIJDLxj1yJY+S80Q3A=" ];
  };

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      nixpkgs,
      flake-utils,
      ...
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        version = (pkgs.lib.importJSON ./.github/config/release-please-manifest.json).".";

        package = pkgs.buildGoModule {
          pname = "protoc-gen-go-defaults";
          inherit version;
          src = pkgs.lib.cleanSource ./.;
          subPackages = [ "cmd/protoc-gen-go-defaults" ];
          vendorHash = "sha256-lV5gsAPaei/R+cRvJTiYXmWgZzvYwYMp2TYbi9FhZTE=";
          ldflags = [ "-s" "-w" "-X main.version=${version}" ];
          meta = with pkgs.lib; {
            description = "A protoc plugin that generates SetDefaults() methods from proto extensions";
            license = licenses.asl20;
            mainProgram = "protoc-gen-go-defaults";
          };
        };
      in
      {
        packages.default = package;

        devShells.default = pkgs.mkShell {
          name = "protoc-gen-go-defaults";
          packages = [
            pkgs.go
            pkgs.protobuf
            pkgs.buf
          ];
        };
      }
    );
}
