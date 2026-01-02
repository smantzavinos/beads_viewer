{
  description = "bv - Terminal UI for the Beads issue tracker";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };

        version = "0.11.3";

        # To update vendorHash after go.mod/go.sum changes:
        # 1. Change vendorHash to: pkgs.lib.fakeHash
        # 2. Run: nix build .#bv 2>&1 | grep "got:"
        # 3. Copy the hash from "got:" and paste it below
        vendorHash = "sha256-rtIqTK6ez27kvPMbNjYSJKFLRbfUv88jq8bCfMkYjfs=";
      in
      {
        packages = {
          bv = pkgs.buildGoModule {
            pname = "bv";
            inherit version;

            src = ./.;

            inherit vendorHash;

            subPackages = [ "cmd/bv" ];

            ldflags = [
              "-s"
              "-w"
              "-X github.com/Dicklesworthstone/beads_viewer/pkg/version.Version=v${version}"
            ];

            meta = with pkgs.lib; {
              description = "Terminal UI for the Beads issue tracker";
              homepage = "https://github.com/Dicklesworthstone/beads_viewer";
              license = licenses.mit;
              maintainers = [ ];
              mainProgram = "bv";
            };
          };

          default = self.packages.${system}.bv;
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            gotools
            go-tools
            delve
          ];

          shellHook = ''
            echo "bv development shell"
            echo "Go version: $(go version)"
          '';
        };
      }
    );
}
