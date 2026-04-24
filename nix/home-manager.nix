self:
{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.services.runix;
  settingsFormat = pkgs.formats.yaml { };
in
{
  options.services.runix = {
    enable = lib.mkEnableOption "Runix process manager";

    package = lib.mkOption {
      type = lib.types.package;
      default = self.packages.${pkgs.stdenv.hostPlatform.system}.default;
      defaultText = lib.literalExpression "self.packages.\${system}.default";
      description = "The runix package to use.";
    };

    settings = lib.mkOption {
      type = lib.types.submodule {
        freeformType = settingsFormat.type;
        options = {
          daemon = lib.mkOption {
            type = lib.types.submodule {
              freeformType = settingsFormat.type;
              options = {
                data_dir = lib.mkOption {
                  type = lib.types.str;
                  default = "${config.home.homeDirectory}/.runix";
                  description = "Runix data directory.";
                };
                socket_path = lib.mkOption {
                  type = lib.types.str;
                  default = "${config.home.homeDirectory}/.runix/tmp/runix.sock";
                  description = "Unix socket path for IPC.";
                };
                log_level = lib.mkOption {
                  type = lib.types.enum [
                    "trace"
                    "debug"
                    "info"
                    "warn"
                    "error"
                    "fatal"
                    "panic"
                  ];
                  default = "info";
                  description = "Log level.";
                };
              };
            };
            default = { };
            description = "Daemon settings.";
          };
        };
      };
      default = { };
      description = "Runix configuration (runix.yaml).";
    };
  };

  config = lib.mkIf cfg.enable {
    home.packages = [ cfg.package ];

    xdg.configFile."runix/runix.yaml".source =
      settingsFormat.generate "runix.yaml" cfg.settings;

    systemd.user.services.runix = {
      Unit = {
        Description = "Runix Process Manager";
        After = [ "default.target" ];
      };

      Service = {
        ExecStart = "${lib.getExe cfg.package} daemon start --config ${config.xdg.configHome}/runix/runix.yaml";
        Restart = "on-failure";
        RestartSec = "5s";
      };

      Install.WantedBy = [ "default.target" ];
    };
  };
}
