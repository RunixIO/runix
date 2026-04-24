{
  description = "Runix — A modern process manager and application supervisor";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    home-manager = {
      url = "github:nix-community/home-manager";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      home-manager,
    }:
    let
      version = self.shortRev or (builtins.substring 0 8 self.lastModifiedDate);

      forEachSystem = nixpkgs.lib.genAttrs [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
    in
    {
      packages = forEachSystem (system: rec {
        default = runix;
        runix = nixpkgs.legacyPackages.${system}.callPackage ./nix/package.nix {
          inherit version;
          src = self;
        };
      });

      devShells = forEachSystem (system: {
        default = nixpkgs.legacyPackages.${system}.mkShell {
          packages = with nixpkgs.legacyPackages.${system}; [
            go
            gopls
            gotools
            golangci-lint
            goreleaser
          ];

          shellHook = ''
            echo "Runix dev shell"
            echo "  Go:    $(go version)"
            echo "  Lint:  $(golangci-lint version --short 2>/dev/null || echo 'n/a')"
            echo ""
            echo "  make build / just build  — build runix"
            echo "  make test  / just test   — run tests"
            echo "  make lint  / just lint   — run linter"
            echo "  make vet   / just vet    — run go vet"
          '';
        };
      });

      overlays.default = final: _prev: {
        runix = self.packages.${final.stdenv.hostPlatform.system}.default;
      };

      formatter = forEachSystem (system: nixpkgs.legacyPackages.${system}.nixfmt-tree);

      nixosModules.default = import ./nix/nixos-module.nix self;
      homeManagerModules.default = import ./nix/home-manager.nix self;
    };
}
