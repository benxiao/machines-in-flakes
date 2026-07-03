{
  description = "all my machines in flakes";
  inputs.nixpkgs.url = "github:nixos/nixpkgs/nixos-26.05";
  inputs.nixpkgs-unstable.url = "github:nixos/nixpkgs/nixpkgs-unstable";
  inputs.nixpkgs-master.url = "github:nixos/nixpkgs/master";

  inputs.nixos-hardware.url = "github:NixOS/nixos-hardware/master";
  inputs.vscode-server.url = "github:msteen/nixos-vscode-server";
  inputs.home-manager = {
    url = "github:nix-community/home-manager/release-26.05";
    inputs.nixpkgs.follows = "nixpkgs";
  };
  outputs = { self, nixpkgs, nixpkgs-unstable, nixpkgs-master, nixos-hardware, vscode-server, home-manager }:
    {
      nixosConfigurations =
        let
          system = "x86_64-linux";
          unstable = import nixpkgs-unstable {
            inherit system;
            config.allowUnfree = true;
          };

          master = import nixpkgs-master {
            inherit system;
            config.allowUnfree = true;
          };

          intelCpuModule = ({ ... }: {
            powerManagement.cpuFreqGovernor = "powersave";
            hardware.cpu.intel.updateMicrocode = true;
          });

          amdCpuModule = ({ ... }: {
            boot.kernelModules = [ "kvm-amd" ];
            hardware.cpu.amd.updateMicrocode = true;
          });

          desktopAppsModule = ({ pkgs, ... }: {
            environment.systemPackages = with pkgs; [
              chromium
              unstable.google-chrome
              audacity
              betterlockscreen
              postman
              # openshot-qt
              ledger-live-desktop
              vscode
              gimp
              jetbrains.goland
              nextcloud-client
              mongodb-compass
              slack
              mendeley
              gnome-boxes
              gnome-text-editor
              baobab
              file-roller
              gnome-system-monitor
              nautilus
              gnome-logs
              gnome-power-manager
              # alacritty
              ghostty
              gnome-chess
              stockfish
              celluloid
              vlc
              firefox
              thunderbird
              tor-browser
              libreoffice
              nomacs
              joplin-desktop
            ];
          });

          googleSDKPackageModule = ({ pkgs, ... }:
            let
              gdk = pkgs.google-cloud-sdk.withExtraComponents
                (with pkgs.google-cloud-sdk.components; [
                  gke-gcloud-auth-plugin
                  pubsub-emulator
                ]);
            in
            {
              environment.systemPackages = [
                gdk
              ];
            });

          makePython3Module =
            { additionalPkgFun ? p: [ ]
            }: ({ pkgs, ... }:
            let
              defaultPkgFun = p: with p; [
                python-lsp-server
                ipython
                pandas
                jupyter
                markitdown
              ] ++ additionalPkgFun (p);

              python3env = unstable.python3.withPackages (defaultPkgFun);
            in
            {
              environment.systemPackages = [
                python3env
              ];
            });

          printerModule = ({ pkgs, ... }: {
            services.printing.enable = true;
            services.avahi.enable = true;
            services.avahi.nssmdns4 = true;
            services.printing.drivers = [ pkgs.brlaser pkgs.brgenml1lpr pkgs.brgenml1cupswrapper ];
            # for a WiFi printer
            services.avahi.openFirewall = false;
          });

          makeServerModule = { allowPassWordAuthentication ? false }: ({ ... }: {
            services.displayManager.gdm.autoSuspend = false;
            security.polkit.extraConfig = ''
              polkit.addRule(function(action, subject) {
                if (action.id == "org.freedesktop.login1.suspend" ||
                  action.id == "org.freedesktop.login1.suspend-multiple-sessions" ||
                  action.id == "org.freedesktop.login1.hibernate" ||
                  action.id == "org.freedesktop.login1.hibernate-multiple-sessions")
                {
                  return polkit.Result.NO;
                }
              });
            '';
            services.openssh = {
              enable = true;
              settings.PasswordAuthentication = allowPassWordAuthentication;
            };
          });

          nvidiaModule = ({ ... }: {
            services.xserver.videoDrivers = [ "nvidia" ];
            hardware.nvidia.nvidiaPersistenced = true;
            hardware.nvidia.open = false;
            hardware.nvidia.modesetting.enable = true;
            hardware.graphics.enable32Bit = true;
            # docker  run  --device=nvidia.com/gpu=0 --rm nvidia/cuda:11.0.3-base-ubuntu20.04 nvidia-smi
            hardware.nvidia-container-toolkit.enable = true;
          });

          # Frigate's continuous NVDEC decode keeps a CUDA context open at all
          # times, which pins the GPU near max boost clock (and ~100-160W)
          # even though actual utilization is <30%. Locking the clock range
          # low is enough for NVDEC (fixed-function hardware) without hurting
          # decode throughput.
          makeGpuClockLockModule = { minClockMHz, maxClockMHz }: ({ config, ... }: {
            systemd.services.nvidia-clock-lock = {
              description = "Lock NVIDIA GPU clocks to reduce idle power draw";
              wants = [ "nvidia-persistenced.service" ];
              after = [ "nvidia-persistenced.service" ];
              wantedBy = [ "multi-user.target" ];
              serviceConfig = {
                Type = "oneshot";
                RemainAfterExit = true;
                ExecStart = "${config.hardware.nvidia.package.bin}/bin/nvidia-smi -lgc ${toString minClockMHz},${toString maxClockMHz}";
                ExecStop = "${config.hardware.nvidia.package.bin}/bin/nvidia-smi -rgc";
              };
            };
          });

          ollamaModule = ({ ... }: {
            services.ollama = {
              enable = true;
              package = unstable.ollama-cuda;
            };
            environment.systemPackages = [ unstable.opencode ];
          });

          makeRouterMonitorModule = { routerIp ? "192.168.30.1" }: ({ pkgs, ... }: {
            systemd.services.router-monitor = {
              description = "Router Monitor Service";
              wants = [ "network-online.target" ];
              after = [ "network-online.target" ];
              wantedBy = [ "multi-user.target" ];
              serviceConfig = {
                ExecStart = pkgs.writeShellScript "router-monitor" ''
                  ROUTER_IP='${routerIp}';
                  MAX_ATTEMPTS=5;
                  SLEEP_INTERVAL=30s;
                  attempt=0;
                  while true; do
                    if ${pkgs.iputils}/bin/ping -c 1 $ROUTER_IP > /dev/null; then
                      attempt=0;
                      echo 'Router is reachable. No action required.';
                    else
                      echo "Router is unreachable. Attempt $attempt of $MAX_ATTEMPTS.";
                      attempt=$((attempt + 1));
                      if [ $attempt -ge $MAX_ATTEMPTS ]; then
                        echo "Router has been unreachable for $MAX_ATTEMPTS attempts. Initiating shutdown.";
                        poweroff
                      fi;
                    fi;
                    sleep $SLEEP_INTERVAL;
                  done
                '';
                Restart = "on-failure";
                Type = "simple";
              };
            };
          });

          makeStorageModule =
            { rootPool ? "zroot/root"
            , bootDevice ? "/dev/nvme0n1p3"
            , swapDevice ? "/dev/nvme0n1p2"
            , extraPools ? [ ]

            }: ({ pkgs, lib, modulesPath, ... }: {
              boot.initrd.availableKernelModules = [ "nvme" ];
              fileSystems."/" = { device = rootPool; fsType = "zfs"; };
              fileSystems."/boot" = { device = bootDevice; fsType = "vfat"; };
              swapDevices = [{ device = swapDevice; }];
              boot.zfs.extraPools = extraPools;
              boot.zfs.forceImportAll = true;
              boot.zfs.forceImportRoot = true;
            });

          # Factory for local Go web services backed by PostgreSQL.
          # Builds the derivation, creates the system user/group, wires up the
          # systemd unit, and declares the postgres database + user — all in one
          # module so adding a new service is a single call-site change.
          makeGoService =
            { pname
            , version
            , src
            , vendorHash
            , description ? pname
            , listenEnvVar
            , listenPort
            , dbDsnEnvVar
            , dbName
            , extraEnv ? { }
            , environmentFile ? null
            }: ({ pkgs, lib, ... }:
            let
              svcUser = builtins.replaceStrings [ "-" ] [ "_" ] pname;
              pkg = pkgs.buildGoModule { inherit pname version src vendorHash; ldflags = [ "-X main.appVersion=${version}" ]; };
            in
            {
              services.postgresql.ensureDatabases = [ dbName ];
              services.postgresql.ensureUsers = [{
                name = svcUser;
                ensureDBOwnership = true;
              }];
              users.users.${svcUser} = {
                isSystemUser = true;
                group = svcUser;
                description = "${pname} service user";
              };
              users.groups.${svcUser} = { };
              systemd.services.${pname} = {
                inherit description;
                wantedBy = [ "multi-user.target" ];
                after = [ "network.target" "postgresql.service" ];
                requires = [ "postgresql.service" ];
                environment = {
                  ${listenEnvVar} = listenPort;
                  ${dbDsnEnvVar} = "host=/run/postgresql dbname=${dbName} user=${svcUser} sslmode=disable";
                } // extraEnv;
                serviceConfig = {
                  ExecStart = "${pkg}/bin/${pname}";
                  Restart = "on-failure";
                  RestartSec = "5s";
                  User = svcUser;
                  Group = svcUser;
                  StateDirectory = pname;
                } // lib.optionalAttrs (environmentFile != null) {
                  EnvironmentFile = environmentFile;
                };
              };
            });

          makeSystemModule =
            { hostName
            , extraModules ? [ ]
            }: {
              inherit system;
              modules =
                [
                  home-manager.nixosModules.home-manager
                  ({ pkgs, lib, config, modulesPath, ... }:
                    {
                      imports = [ (modulesPath + "/installer/scan/not-detected.nix") ];
                      nix.extraOptions = "experimental-features = nix-command flakes";
                      nixpkgs.config.allowUnfree = true;

                      hardware.bluetooth.enable = true;
                      hardware.ledger.enable = true;

                      boot.loader.systemd-boot.enable = true;

                      boot.extraModprobeConfig = ''
                          options zfs zfs_arc_max=8884901888
                        '';
                      boot.loader.efi.canTouchEfiVariables = true;

                      # hostId is set per-machine below; run `head -c 8 /etc/machine-id` on each host
                      networking.hostName = hostName;
                      time.timeZone = "Australia/Melbourne";

                      # services.logind.extraConfig = ''
                      #   RuntimeDirectorySize=10G
                      # '';

                      i18n.defaultLocale = "en_AU.UTF-8";
                      services.gnome.core-apps.enable = false;
                      services.gnome.localsearch.enable = false;
                      services.gnome.tinysparql.enable = false;
                      services.gnome.gnome-remote-desktop.enable = false;
                      services.xserver.enable = true;
                      services.desktopManager.gnome.enable = true;
                      services.displayManager.gdm.enable = true;
                      services.libinput.enable = true;
                      services.pcscd.enable = true;
                      services.tailscale.enable = true;
                      services.zfs.trim.enable = true;
                      # enable gpg
                      programs.gnupg.agent = {
                        enable = true;
                        pinentryPackage = pkgs.pinentry-curses;
                        enableSSHSupport = true;
                      };

                      programs.gnome-disks.enable = true;
                      environment.systemPackages = with pkgs; [
                        bartib
                        btop
                        nix-index
                        nix-init
                        nix-tree
                        nixpkgs-review
                        nil
                        nixpkgs-fmt
                        unstable.ffmpeg-full
                        docker-compose
                        openssl
                        git
                        git-lfs
                        pinentry-curses
                        bottom
                        master.claude-code
                        iotop
                        broot
                        bandwhich
                        zellij
                        tokei
                        choose
                        fd
                        rnr
                        tealdeer
                        lm_sensors
                        smartmontools
                        nmap
                        unzip
                        pv
                        ouch
                        silver-searcher
                        master.helix
                        wget
                        tig
                        xclip
                        tree
                        atop
                        go
                        gcc
                        golangci-lint
                        gopls
                        go-mockery
                        sysstat
                      ];
                      users.users.rxiao = {
                        isNormalUser = true;
                        extraGroups = [ "wheel" "docker" ];
                      };
                      virtualisation.libvirtd.enable = true;
                      virtualisation.docker = {
                        enable = true;
                        storageDriver = "zfs";
                        liveRestore = false;
                        daemon.settings = {
                          dns = [ "8.8.8.8" "1.1.1.1" ];
                        };
                      };
                      systemd.services.docker.wants = [ "zfs-zed.service" ];
                      systemd.services.docker.after = [ "zfs-zed.service" ];
                      hardware.graphics.enable = true;
                      networking.firewall.enable = false;
                      system.stateVersion = "25.05";

                      home-manager.useGlobalPkgs = true;
                      home-manager.useUserPackages = true;
                      home-manager.users.rxiao = { pkgs, ... }: {
                        programs.bash = {
                          enable = true;
                          initExtra = ''
                            ${pkgs.fastfetch}/bin/fastfetch -l small
                          '';
                          sessionVariables = {
                            RUST_BACKTRACE = "1";
                            EDITOR = "hx";
                          };
                          shellAliases = {
                            nix-generations = "nix profile history --profile /nix/var/nix/profiles/system";
                            athena = "ssh rxiao@athena.pinto-stargazer.ts.net";
                          };
                        };
                        home.stateVersion = "25.05";
                      };
                    })

                ] ++ extraModules;
            };
        in
        {
          # Lenovo T490
          apollo = nixpkgs.lib.nixosSystem (makeSystemModule {
            hostName = "apollo";
            extraModules = [
              (makeStorageModule { })
              ({ pkgs, lib, modulesPath, ... }:
                {
                  networking.hostId = "00000000"; # replace: run `head -c 8 /etc/machine-id` on apollo
                  environment.systemPackages = with pkgs; [
                    asunder
                  ];
                })
              intelCpuModule
              printerModule
              (makePython3Module {
                additionalPkgFun = p: [
                  # p.langchain-community
                  # p.langchain
                  # p.langchain-chroma
                ];
              })
              desktopAppsModule
              googleSDKPackageModule
              nixos-hardware.nixosModules.lenovo-thinkpad-t490
            ];

          });
          # amd ryzen 7 3700x
          athena = nixpkgs.lib.nixosSystem (makeSystemModule {
            hostName = "athena";
            extraModules = [
              ({ pkgs, lib, config, modulesPath, ... }:
                let
                  drive-monitor = pkgs.buildGoModule {
                    pname = "drive-monitor";
                    version = "0.1.0";
                    src = ./drive-monitor;
                    vendorHash = null;
                  };
                in
                {
                  networking.hostId = "00000000"; # replace: run `head -c 8 /etc/machine-id` on athena
                  services.vscode-server.enable = true;

                  # Headless server — no GPU or monitor attached
                  services.xserver.enable = lib.mkForce false;
                  services.displayManager.gdm.enable = lib.mkForce false;
                  services.desktopManager.gnome.enable = lib.mkForce false;

                  services.postgresql = {
                    enable = true;
                    package = pkgs.postgresql_16;
                    authentication = pkgs.lib.mkOverride 10 ''
                      local  all  all              trust
                      host   all  all  127.0.0.1/32  trust
                      host   all  all  ::1/128       trust
                    '';
                  };

                  systemd.services.restart-broken-containers-after-reboot = {
                    wantedBy = [ "multi-user.target" ];
                    after = [
                      "docker.service"
                      "zfs-import-blue2t.service"
                      "zfs-import-c7.service"
                      "zfs-import-exos12.service"
                      "zfs-import-exos16.service"
                    ];
                    requires = [
                      "docker.service"
                      "zfs-import-blue2t.service"
                      "zfs-import-c7.service"
                      "zfs-import-exos12.service"
                      "zfs-import-exos16.service"
                    ];
                    serviceConfig.Type = "oneshot";
                    script = ''
                      for app in nut
                      do
                        if ! cd /home/rxiao/$app 2>/dev/null; then
                          echo "$app: directory not found, skipping"
                          continue
                        fi
                        echo "$app: restarting..."
                        ${pkgs.docker-compose}/bin/docker-compose down || true
                        if ! ${pkgs.docker-compose}/bin/docker-compose up -d; then
                          echo "$app: failed to start (check docker-compose config)"
                        fi
                      done
                    '';
                  };
                  systemd.services.drive-monitor = {
                    description = "Drive Health Monitor";
                    wantedBy = [ "multi-user.target" ];
                    after = [ "network.target" ];
                    path = [ pkgs.smartmontools pkgs.zfs ];
                    serviceConfig = {
                      ExecStart = "${drive-monitor}/bin/drive-monitor";
                      Restart = "on-failure";
                      RestartSec = "5s";
                    };
                  };
                })
              (makeGoService {
                pname = "fpv-manager";
                version = "0.8.0";
                src = ./fpv-manager;
                vendorHash = "sha256-Qs23BHgrlK0P5BREEzS5Y/2G7mL1pcSd1k3z8NUw/mM=";
                description = "FPV Drone Inventory Manager";
                listenEnvVar = "FPV_LISTEN";
                listenPort = ":10091";
                dbDsnEnvVar = "FPV_DB_DSN";
                dbName = "fpv_manager";
                extraEnv = {
                  FPV_VIDEO_DIR = "/var/lib/fpv-manager/videos";
                  FPV_FFMPEG = "${unstable.ffmpeg-full}/bin/ffmpeg";
                };
              })
              (makeGoService {
                pname = "kanban";
                version = "0.1.0";
                src = ./kanban;
                # NOTE: this hash matches fpv-manager's — verify with vendorHash = null if kanban deps changed
                vendorHash = "sha256-Qs23BHgrlK0P5BREEzS5Y/2G7mL1pcSd1k3z8NUw/mM=";
                description = "Kanban Board";
                listenEnvVar = "KANBAN_LISTEN";
                listenPort = ":10092";
                dbDsnEnvVar = "KANBAN_DB_DSN";
                dbName = "kanban";
                extraEnv = {
                  KANBAN_UPLOAD_DIR = "/var/lib/kanban/uploads";
                };
              })
              (makeGoService {
                pname = "filebrowser";
                version = "1.17.2";
                src = ./filebrowser;
                vendorHash = "sha256-cCSZsNYMmjh48YiztNTpUrqmDdL1OehYBfZm3evU9l8=";
                description = "File Browser";
                listenEnvVar = "FB_LISTEN";
                listenPort = ":10094";
                dbDsnEnvVar = "FB_DB_DSN";
                dbName = "filebrowser";
                extraEnv = {
                  FB_FFMPEG = "${unstable.ffmpeg-full}/bin/ffmpeg";
                  FB_MARKITDOWN = "${unstable.python3Packages.markitdown}/bin/markitdown";
                };
                # Create filebrowser/.env (gitignored) with:
                #   FB_ADMIN_USERNAME=admin
                #   FB_ADMIN_PASSWORD=your-password
                environmentFile = "/home/rxiao/machines-in-flakes/filebrowser/.env";
              })
              ({ lib, ... }: {
                # run as rxiao so it can read user-owned files and directories
                systemd.services.filebrowser.serviceConfig.User = lib.mkForce "rxiao";
              })
              (makeRouterMonitorModule { })
              nvidiaModule
              (makeGpuClockLockModule { minClockMHz = 300; maxClockMHz = 900; })
              (makeStorageModule {
                extraPools = [ "blue2t" "c7" "exos12" "exos16" "tm1t" ];
              })
              (makeServerModule { })
              amdCpuModule
              vscode-server.nixosModule
              (makePython3Module { })
              googleSDKPackageModule
            ];
          });
          # intel 13500
          wotan = nixpkgs.lib.nixosSystem (makeSystemModule {
            hostName = "wotan";
            extraModules = [
              intelCpuModule
              printerModule
              googleSDKPackageModule
              desktopAppsModule
              (makePython3Module { })
              (makeServerModule { })
              nvidiaModule
              (makeStorageModule {
                swapDevice = "/dev/nvme2n1p2";
                bootDevice = "/dev/disk/by-uuid/DED6-AF46";
                extraPools = [ "wotan" ];
              })
              ({ ... }: {
                networking.hostId = "00000000"; # replace: run `head -c 8 /etc/machine-id` on wotan
                home-manager.users.rxiao = { ... }: {
                  programs.bash.shellAliases = {
                    mendeley = "mendeley-reference-manager --no-sandbox";
                  };
                };
              })
              ollamaModule
            ];
          });
          # amd ryzen 3950x
          dante = nixpkgs.lib.nixosSystem (makeSystemModule {
            hostName = "dante";
            extraModules = [
              ({ pkgs, lib, modulesPath, ... }:
                {
                  networking.hostId = "00000000"; # replace: run `head -c 8 /etc/machine-id` on dante
                  environment.systemPackages = with pkgs; [
                    audacity
                    postgresql
                    go-migrate
                    temporal
                    temporal-cli
                  ];
                  programs.steam = {
                    enable = true;
                    remotePlay.openFirewall = true; # Open ports in the firewall for Steam Remote Play
                    dedicatedServer.openFirewall = true; # Open ports in the firewall for Source Dedicated Server
                  };
                  home-manager.users.rxiao = { ... }: {
                    programs.bash.sessionVariables = {
                      MOZ_ENABLE_WAYLAND = "0";
                    };
                  };
                })
              (makeStorageModule {
                swapDevice = "/dev/disk/by-uuid/45c86fa9-ddbf-45c6-96a6-220fac48667c";
                bootDevice = "/dev/disk/by-uuid/B9A1-4A5D";
                extraPools = [ "dante" "tank" ];
              })
              amdCpuModule
              printerModule
              (makeServerModule { })
              nvidiaModule
              (makePython3Module { })
              desktopAppsModule
              googleSDKPackageModule
            ];
          });
        };
    };
}
