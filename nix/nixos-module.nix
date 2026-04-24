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
                  default = "/var/lib/runix";
                  description = "Runix data directory.";
                };
                socket_path = lib.mkOption {
                  type = lib.types.str;
                  default = "/run/runix/runix.sock";
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

    configDir = lib.mkOption {
      type = lib.types.str;
      default = "/etc/runix";
      description = "Directory for runix.yaml config file.";
    };
  };

  config = lib.mkIf cfg.enable {
    users.users.runix = {
      isSystemUser = true;
      group = "runix";
      home = cfg.settings.daemon.data_dir;
      createHome = true;
    };

    users.groups.runix = { };

    environment.etc."runix/runix.yaml".source =
      settingsFormat.generate "runix.yaml" cfg.settings;

    systemd.services.runix = {
      description = "Runix Process Manager";
      wantedBy = [ "multi-user.target" ];
      after = [ "network.target" ];

      serviceConfig = {
        Type = "notify";
        ExecStart = "${lib.getExe cfg.package} daemon start --config ${cfg.configDir}/runix.yaml";
        Restart = "on-failure";
        RestartSec = "5s";

        User = "runix";
        Group = "runix";

        RuntimeDirectory = "runix";
        RuntimeDirectoryMode = "0750";
        StateDirectory = "runix";
        WorkingDirectory = cfg.settings.daemon.data_dir;

        LimitNOFILE = "65536";
        NoNewPrivileges = true;
        ProtectSystem = "strict";
        ProtectHome = true;
        PrivateTmp = true;
      };
    };
  };
}
