{
  description = "all my machines in flakes";
  inputs.nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
  inputs.nixpkgs-legacy.url = "github:nixos/nixpkgs/nixos-23.05";
  inputs.nixpkgs-stable.url = "github:nixos/nixpkgs/nixos-23.11";
  inputs.nixos-hardware.url = "github:NixOS/nixos-hardware/master";
  inputs.vscode-server.url = "github:msteen/nixos-vscode-server";
  outputs = { self, nixpkgs, nixpkgs-stable, nixpkgs-legacy, nixos-hardware, vscode-server }:
    {
      nixosConfigurations =
        let
          system = "x86_64-linux";
          stable = import nixpkgs-stable {
            inherit system;
            config.allowUnfree = true;
          };

          legacy = import nixpkgs-legacy {
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
              audacity
              betterlockscreen
              legacy.postman
              openshot-qt
              ledger-live-desktop
              vscode
              gimp
              jetbrains.goland
              nextcloud-client
              mongodb-compass
              slack
              mendeley
              gnome.gnome-boxes
              gnome-text-editor
              gnome.baobab
              gnome.file-roller
              gnome.gnome-system-monitor
              gnome.nautilus
              gnome.gnome-logs
              gnome.gnome-power-manager
              alacritty
              gnome.gnome-chess
              stockfish
              celluloid
              vlc
              firefox
              opera
              thunderbird
              tor-browser-bundle-bin
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

          printerModule = ({ pkgs, ... }: {
            services.printing.enable = true;
            services.avahi.enable = true;
            services.avahi.nssmdns4 = true;
            services.printing.drivers = [ pkgs.brlaser pkgs.brgenml1lpr pkgs.brgenml1cupswrapper ];
            # for a WiFi printer
            services.avahi.openFirewall = true;
          });

          makeServerModule = { allowPassWordAuthentication ? false }: ({ ... }: {
            services.xserver.displayManager.gdm.autoSuspend = false;
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

          makeNvidiaModule = { powerlimit }: ({ ... }: {
            services.xserver.videoDrivers = [ "nvidia" ];
            hardware.nvidia.nvidiaPersistenced = true;
            hardware.nvidia.modesetting.enable = true;
            virtualisation.docker.enableNvidia = true;
            hardware.opengl.driSupport32Bit = true;
            systemd.services.nvidia-power-limiter = {
              wantedBy = [ "multi-user.target" ];
              description = "set power limit for nvidia gpus";
              serviceConfig = {
                Type = "simple";
                ExecStart = ''
                  /run/current-system/sw/bin/nvidia-smi -i 0 -pl ${builtins.toString powerlimit}
                '';
              };
            };
          });

          checkRouterAliveModule = { pkgs, ... }: {
            systemd.services.router-monitor = {
              description = "Router Monitor Service";
              after = [ "network.target" ];
              wantedBy = [ "multi-user.target" ];
              serviceConfig = {
                ExecStart = pkgs.writeShellScript "router-monitor" ''
                  ROUTER_IP='192.168.50.1';
                  MAX_ATTEMPTS=5;
                  SLEEP_INTERVAL=10s;
                  attempt=0;
                  while true; do
                    if ${pkgs.iputils}/bin/ping -c 1 $ROUTER_IP > /dev/null; then
                      attempt=0;
                      echo 'Router is reachable. No action required.';
                    else
                      echo "Router is unreachable. Attempt $attempt of $MAX_ATTEMPTS.";
                      attempt=$((attempt + 1));
                      if [ $attempt -ge $MAX_ATTEMPTS ]; then
                        echo 'Router has been unreachable for $MAX_ATTEMPTS attempts. Initiating shutdown.';
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
          };

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
            });

          basicSystemModule =
            { hostName
            , extraModules ? [ ]
            }: {
              inherit system;
              modules =
                [
                  ({ pkgs, lib, config, modulesPath, ... }:
                    {
                      imports = [ (modulesPath + "/installer/scan/not-detected.nix") ];
                      nix.extraOptions = "experimental-features = nix-command flakes";
                      # sound
                      sound.enable = true;
                      nixpkgs.config.pulseaudio = true;
                      hardware.pulseaudio.enable = true;
                      hardware.pulseaudio.support32Bit = true;
                      hardware.bluetooth.enable = true;
                      hardware.ledger.enable = true;
                      nixpkgs.config.allowUnfree = true;

                      boot.loader.systemd-boot.enable = true;
                      boot.kernelPackages = config.boot.zfs.package.latestCompatibleLinuxPackages;
                      boot.loader.efi.canTouchEfiVariables = true;

                      networking.hostId = "00000000";
                      networking.hostName = hostName;
                      time.timeZone = "Australia/Melbourne";

                      services.logind.extraConfig = ''
                        RuntimeDirectorySize=10G
                      '';

                      i18n.defaultLocale = "en_AU.UTF-8";
                      services.gnome.core-utilities.enable = false;
                      services.gnome.tracker-miners.enable = false;
                      services.gnome.tracker.enable = false;
                      services.gnome.gnome-remote-desktop.enable = false;
                      services.xserver.enable = true;
                      services.xserver.desktopManager.gnome.enable = true;
                      services.xserver.displayManager.gdm.enable = true;
                      services.libinput.enable = true;
                      services.xserver.xkb.options = "caps:none";
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
                        nix-index
                        nix-init
                        nix-tree
                        nixpkgs-review
                        nil
                        nixpkgs-fmt
                        ffmpeg_6-full
                        docker-compose
                        openssl
                        git
                        git-lfs
                        pinentry-curses
                        bottom
                        iotop
                        broot
                        bandwhich
                        zellij
                        tokei
                        choose
                        fd
                        tealdeer
                        lm_sensors
                        smartmontools
                        nmap
                        unzip
                        pv
                        ouch
                        silver-searcher
                        helix
                        wget
                        tig
                        xclip
                        tree
                        atop
                        go
                        golangci-lint
                        gopls
                        go-mockery
                        sysstat
                      ];
                      environment.variables.EDITOR = "hx";
                      users.users.rxiao = {
                        isNormalUser = true;
                        extraGroups = [ "wheel" "docker" ];
                      };
                      virtualisation.libvirtd.enable = true;
                      virtualisation.docker.enable = true;
                      virtualisation.docker.storageDriver = "zfs";
                      virtualisation.docker.liveRestore = false;
                      hardware.opengl.enable = true;
                      networking.firewall.enable = false;
                      system.stateVersion = "24.05";
                    })

                ] ++ extraModules;
            };

        in
        {
          # Lenovo T490
          apollo = nixpkgs.lib.nixosSystem (basicSystemModule {
            hostName = "apollo";
            extraModules = [
              (makeStorageModule { })
              ({ pkgs, lib, modulesPath, ... }:
                {
                  environment.systemPackages = with pkgs; [
                    asunder
                  ];
                  services.xserver.displayManager.gdm.wayland = false;
                  environment.interactiveShellInit = ''
                    alias athena='ssh rxiao@athena.pinto-stargazer.ts.net'
                    export RUST_BACKTRACE=1
                  '';

                })
              intelCpuModule
              printerModule
              desktopAppsModule
              googleSDKPackageModule
              nixos-hardware.nixosModules.lenovo-thinkpad-t490
            ];

          });
          # amd ryzen 7 3700x
          athena = nixpkgs.lib.nixosSystem (basicSystemModule {
            hostName = "athena";
            extraModules = [
              ({ pkgs, lib, config, modulesPath, ... }:
                {
                  environment.interactiveShellInit = ''
                    function slide-show(){
                      feh -Y -x -q -D 100 -B black -F -Z -z -r "$@";
                    }
                  '';

                  services.vscode-server.enable = true;
                })
              checkRouterAliveModule
              (makeNvidiaModule { powerlimit = 125; })
              (makeStorageModule {
                extraPools = [ "blue2t" "bigdisk"  "exos16" ];
              })
              (makeServerModule { allowPassWordAuthentication = false; })
              amdCpuModule
              vscode-server.nixosModule
              googleSDKPackageModule
            ];
          });
          # intel 13500
          wotan = nixpkgs.lib.nixosSystem (basicSystemModule {
            hostName = "wotan";
            extraModules = [
              intelCpuModule
              printerModule
              googleSDKPackageModule
              desktopAppsModule
              (makeServerModule { })
              (makeNvidiaModule { powerlimit = 205; })
              (makeStorageModule {
                swapDevice = "/dev/nvme2n1p2";
                bootDevice = "/dev/disk/by-uuid/DED6-AF46";
                extraPools = [ "wotan" ];
              })
              ({ pkgs, ... }: {
                environment.systemPackages = with pkgs; [ openshot-qt ];
                environment.interactiveShellInit = ''
                  alias athena='ssh rxiao@athena.pinto-stargazer.ts.net'
                  alias mendeley='mendeley-reference-manager --no-sandbox' 
                  export RUST_BACKTRACE=1
                '';

                programs.steam = {
                  enable = true;
                  remotePlay.openFirewall = true; # Open ports in the firewall for Steam Remote Play
                  dedicatedServer.openFirewall = true; # Open ports in the firewall for Source Dedicated Server
                };
              })
            ];
          });
          # amd ryzen 3950x
          dante = nixpkgs.lib.nixosSystem (basicSystemModule {
            hostName = "dante";
            extraModules = [
              ({ pkgs, lib, modulesPath, ... }:
                {
                  environment.interactiveShellInit = ''
                    alias athena='ssh rxiao@athena.pinto-stargazer.ts.net'
                    export RUST_BACKTRACE=1
                  '';

                  environment.systemPackages = with pkgs; [
                    audacity
                    jetbrains.rust-rover
                    near-cli
                    temporal
                    temporal-cli
                  ];
                  programs.steam = {
                    enable = true;
                    remotePlay.openFirewall = true; # Open ports in the firewall for Steam Remote Play
                    dedicatedServer.openFirewall = true; # Open ports in the firewall for Source Dedicated Server
                  };
                })
              (makeStorageModule {
                swapDevice = "/dev/disk/by-uuid/45c86fa9-ddbf-45c6-96a6-220fac48667c";
                bootDevice = "/dev/disk/by-uuid/B9A1-4A5D";
                extraPools = [ "zdata" "blue3" "dante" "tank" ];
              })
              amdCpuModule
              printerModule
              (makeServerModule {
                allowPassWordAuthentication = false;
              })
              (makeNvidiaModule {
                powerlimit = 205;
              })
              desktopAppsModule
              googleSDKPackageModule
            ];
          });
        };
    };
}
