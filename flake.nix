{
  description = "A simple flake utilizing flake-utils";

  inputs = {
    nixpkgs.url = "nixpkgs/nixos-unstable";
    
    # in the nix global registry: `github:numtide/flake-utils`
    utils.url = "flake-utils";
  };

  outputs = { self, nixpkgs, utils }: utils.lib.eachDefaultSystem (system:
    let
      pkgs = import nixpkgs {
        inherit system;
        # config.allowUnfree = true;
      };
    in {

      # `devShell` or `devShells.default`
      devShell = pkgs.mkShell {
        # Add any you need here
        packages = with pkgs; [ 
          go
        ];

        # Set any environment variables for your dev shell
        env = {
        };

        # Add any shell logic you want executed any time the environment is activated
        shellHook = ''
        '';
      };


      packages = rec {
        hello = pkgs.hello;
        default = hello;
      };

    }
  );

}
