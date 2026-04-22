{
  description = "store — dotfile symlink manager (one repo, one config, one command per machine)";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        # Bump alongside every tagged release; the value is burned into
        # `store --version` via the ldflags below.
        version = "2.3.0";
      in
      {
        packages = {
          default = self.packages.${system}.store;

          store = pkgs.buildGoModule {
            pname = "store";
            inherit version;

            src = ./.;

            # Replace with the hash printed on first `nix build`. We cannot
            # precompute this in a sandbox without running nix, so it ships
            # as a fakeHash sentinel.
            vendorHash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";

            ldflags = [
              "-s"
              "-w"
              "-X main.version=v${version}"
            ];

            subPackages = [ "cmd/store" ];

            # Install shell completions the same way the AUR package does
            # so `store` feels native on NixOS.
            postInstall = ''
              installShellCompletion --cmd store \
                --bash <($out/bin/store completion bash) \
                --fish <($out/bin/store completion fish) \
                --zsh  <($out/bin/store completion zsh)
            '';

            nativeBuildInputs = [ pkgs.installShellFiles ];

            meta = with pkgs.lib; {
              description = "Dotfile symlink manager — one repo, one config, one command per machine";
              homepage = "https://github.com/cushycush/store";
              license = licenses.mit;
              mainProgram = "store";
              platforms = platforms.unix ++ platforms.windows;
            };
          };
        };

        apps.default = flake-utils.lib.mkApp {
          drv = self.packages.${system}.store;
        };

        devShells.default = pkgs.mkShell {
          # Mirror the toolchain CI uses so local iteration matches release.
          packages = [
            pkgs.go_1_26
            pkgs.gopls
            pkgs.gotools
          ];
        };
      });
}
