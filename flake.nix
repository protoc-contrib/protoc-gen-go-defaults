{
  description = "protoc-gen-go-defaults - A protoc plugin that generates Default() methods from proto extensions";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    { nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        version = (pkgs.lib.importJSON ./.github/config/release-please-manifest.json).".";
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "protoc-gen-go-defaults";
          inherit version;
          src = pkgs.lib.cleanSource ./.;
          subPackages = [ "cmd/protoc-gen-go-defaults" ];
          vendorHash = "sha256-vuNTvxyTUIVOsXlwHCoQjJgWZBi/6EA8HKIkv6NWFjc=";
          ldflags = [ "-s" "-w" "-X main.version=${version}" ];
          meta = with pkgs.lib; {
            description = "A protoc plugin that generates Default() methods from proto extensions";
            license = licenses.asl20;
            mainProgram = "protoc-gen-go-defaults";
          };
        };

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
