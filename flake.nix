{
  description = "juggle - minimal agent loop runner";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        version = self.shortRev or self.dirtyShortRev or "dev";
      in {
        packages = {
          juggle = pkgs.buildGoModule {
            pname = "juggle";
            inherit version;
            src = self;
            subPackages = [ "cmd/juggle" ];
            vendorHash = null;
            ldflags = [ "-s" "-w" "-X main.version=${version}" ];
          };
          default = self.packages.${system}.juggle;
        };

        apps = {
          juggle = {
            type = "app";
            program = "${self.packages.${system}.juggle}/bin/juggle";
          };
          default = self.apps.${system}.juggle;
        };
      }
    );
}
