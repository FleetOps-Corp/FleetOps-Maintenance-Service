{pkgs ? import (import ./lon.nix).nixpkgs {}}:
pkgs.mkShellNoCC {
  packages = [
    pkgs.nil
    pkgs.alejandra
    pkgs.go
    pkgs.gofumpt
    pkgs.gopls
    pkgs.golangci-lint
  ];
}
