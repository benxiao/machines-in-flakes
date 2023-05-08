{
  description = "all my machines in flakes";
  inputs.nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
  inputs.nixpkgs-stable.url = "github:nixos/nixpkgs/nixos-22.11";
  inputs.vscode-server.url = "github:msteen/nixos-vscode-server";
  outputs = { self, nixpkgs, nixpkgs-stable, vscode-server }:
    {
      nixosConfigurations =
        let
          system = "x86_64-linux";
          stable = import nixpkgs-stable {
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
              microsoft-edge
              ledger-live-desktop
              vscode
              gimp
              dropbox
              nextcloud-client
              mongodb-compass
              slack
              jetbrains.goland
              mendeley
              jetbrains.pycharm-community
              gnome-text-editor
              gnome.baobab
              gnome.file-roller
              gnome.gnome-system-monitor
              gnome.nautilus
              gnome.gnome-power-manager
              gnome-console
              gnome.gnome-chess
              stockfish
              amberol
              # elisa
              # mpv
              celluloid
              stable.vlc
              firefox
              thunderbird
              tor-browser-bundle-bin
              stable.libreoffice
              qbittorrent
              github-desktop
              nomacs
              joplin-desktop
              android-studio
            ];
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
            virtualisation.docker.enableNvidia = true;
            hardware.opengl.driSupport32Bit = true;
            # systemd.enableUnifiedCgroupHierarchy = false;
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

          simplesystem =
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
                      hardware.ledger.enable = true;
                      nixpkgs.config.allowUnfree = true;
                      boot.loader.systemd-boot.enable = true;
                      # zfs
                      boot.kernelPackages = config.boot.zfs.package.latestCompatibleLinuxPackages;
                      services.zfs.trim.enable = true;

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
                      services.xserver.libinput.enable = true;
                      services.xserver.xkbOptions = "caps:none";
                      services.pcscd.enable = true;
                      services.tailscale.enable = true;
                      # enable gpg
                      programs.gnupg.agent = {
                        enable = true;
                        pinentryFlavor = "curses";
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
                        btop
                        docker-compose
                        git
                        pinentry-curses
                        htop
                        tmux
                        zellij
                        lm_sensors
                        smartmontools
                        nmap
                        unzip
                        silver-searcher
                        helix
                        black
                        wget
                        tig
                        xclip
                        nodejs
                        rustup
                        clang
                        atop
                        rust-analyzer
                        gopls
                        sysstat
                      ];
                      environment.variables.EDITOR = "hx";
                      users.users.rxiao = {
                        isNormalUser = true;
                        extraGroups = [ "wheel" "docker" "vboxusers" ];
                      };
                      virtualisation.libvirtd.enable = true;
                      virtualisation.docker.enable = true;
                      virtualisation.docker.storageDriver = "zfs";
                      virtualisation.docker.liveRestore = false;
                      hardware.opengl.enable = true;
                      networking.firewall.enable = false;
                      system.stateVersion = "22.05"; # Did you read the comment?
                    })

                ] ++ extraModules;
            };
        in
        {
          # Lenovo T490
          apollo = nixpkgs.lib.nixosSystem (simplesystem {
            hostName = "apollo";
            extraModules = [
              (makeStorageModule { })
              ({ pkgs, lib, modulesPath, ... }:
                {
                  services.xserver.displayManager.gdm.wayland = false;
                  environment.systemPackages = with pkgs; [ android-studio ];
                  environment.interactiveShellInit = ''
                    alias athena='ssh rxiao@192.168.50.144'
                    alias wotan='ssh rxiao.asuscomm.com -p 14285'
                    alias artemis='ssh rxiao@artemis.silverpond.com.au'
                    export RUST_BACKTRACE=1
                  '';
                })
              intelCpuModule
              desktopAppsModule
            ];

          });
          # amd ryzen 7 1700
          athena = nixpkgs.lib.nixosSystem (simplesystem {
            hostName = "athena";
            extraModules = [
              ({ pkgs, lib, config, modulesPath, ... }:
                {
                  services.vscode-server.enable = true;
                  environment.systemPackages = with pkgs; [ nethogs qbittorrent-nox ];
                  systemd.services.qbittorrent-server = {
                    wantedBy = [ "multi-user.target" ];
                    description = "qbittorrent webserver";
                    serviceConfig = {
                      Type = "simple";
                      ExecStart = ''
                        ${pkgs.qbittorrent-nox}/bin/qbittorrent-nox --webui-port=8083 
                      '';
                      ExecStop = ''
                        kill -9 $(ps aux | grep qbittorrent | awk '{print $2}' | head -1)
                      '';

                      TimeoutStopSec = "30";
                    };
                  };

                  services.nextcloud = {
                    enable = true;
                    package = pkgs.nextcloud26;
                    enableBrokenCiphersForSSE = false;
                    hostName = "athena";
                    config.extraTrustedDomains = [ "*.*.*.*" ];
                    # https = true;
                    maxUploadSize = "20G";
                    config.adminpassFile = "${pkgs.writeText "adminpass" "rxiao"}";
                  };
                  # security.acme.acceptTerms = true;
                  # services.nginx.virtualHosts.${config.services.nextcloud.hostName} = {
                  #   forceSSL = true;
                  #   enableACME = true;
                  # };
                })
              desktopAppsModule
              (makeStorageModule {
                extraPools = [ "ssd0" "rock" "red4" "exos12" ];
              })
              (makeServerModule { })
              amdCpuModule
              vscode-server.nixosModule
              (makeNvidiaModule { powerlimit = 75; })
            ];
          });
          # amd ryzen 7 3700x
          wotan = nixpkgs.lib.nixosSystem (simplesystem {
            hostName = "wotan";
            extraModules = [
              intelCpuModule
              desktopAppsModule
              (makeServerModule { })
              (makeNvidiaModule { powerlimit = 205; })
              (makeStorageModule {
                swapDevice = "/dev/disk/by-uuid/283d3cd3-03c7-4e18-97f0-d003cfe1d4c0";
                bootDevice = "/dev/disk/by-uuid/C967-EF6D";
                extraPools = [ "black1" ];
              })
            ];
          });
          # amd ryzen 3950x
          dante = nixpkgs.lib.nixosSystem (simplesystem {
            hostName = "dante";
            extraModules = [
              ({ pkgs, lib, modulesPath, ... }:
                {
                  environment.interactiveShellInit = ''
                    alias athena='ssh rxiao@192.168.50.144'
                    alias artemis='ssh rxiao@artemis.silverpond.com.au'
                    alias mendeley='mendeley-reference-manager --no-sandbox' 
                    export RUST_BACKTRACE=1
                  '';
                  services.openssh = {
                    enable = true;
                    settings.PasswordAuthentication = false;
                  };
                  # virtualisation.virtualbox.host.enable = true;
                  # virtualisation.virtualbox.host.enableExtensionPack = true;
                  # users.extraGroups.vboxusers.members = [ "user-with-access-to-virtualbox" ];
                  programs.steam = {
                    enable = true;
                    remotePlay.openFirewall = true; # Open ports in the firewall for Steam Remote Play
                    dedicatedServer.openFirewall = true; # Open ports in the firewall for Source Dedicated Server
                  };
                })
              (makeStorageModule {
                extraPools = [ "zdata" "blue3" "tt1t" ];
              })
              amdCpuModule
              (makeNvidiaModule {
                powerlimit = 205;
              })
              desktopAppsModule
            ];
          });
        };
    };
}
