{
  description = "all my machines in flakes";
  inputs.nixpkgs.url = "github:nixos/nixpkgs/nixos-22.05";
  inputs.nixpkgs-master.url = "github:nixos/nixpkgs";
  inputs.vscode-server.url = "github:msteen/nixos-vscode-server";
  outputs = { self, nixpkgs, nixpkgs-master, vscode-server }:
    {
      nixosConfigurations =
        let
          system = "x86_64-linux";
          master = import nixpkgs-master {
            inherit system;
            config.allowUnfree = true;
          };

          intelCpuModule = ({ pkgs, lib, modulesPath, ... }: {
            powerManagement.cpuFreqGovernor = "powersave";
            hardware.cpu.intel.updateMicrocode = true;
          });


          amdCpuModule = ({ pkgs, lib, modulesPath, ... }: {
            boot.kernelModules = [ "kvm-amd" ];
            hardware.cpu.amd.updateMicrocode = true;
          });

          makeNvidiaModule = { powerlimit }: ({ pkgs, lib, modulesPath, ... }: {
            services.xserver.videoDrivers = [ "nvidia" ];
            hardware.nvidia.nvidiaPersistenced = true;
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

          makeStorageModule =
            { rootPool ? "zroot/root"
            , bootDevice ? "/dev/nvme0n1p3"
            , swapDevice ? "/dev/nvme0n1p2"
            , postDeviceCommands ? ""
            }: ({ pkgs, lib, modulesPath, ... }: {

              boot.initrd.availableKernelModules = [ "nvme" ];
              fileSystems."/" = { device = rootPool; fsType = "zfs"; };
              fileSystems."/boot" = { device = bootDevice; fsType = "vfat"; };
              swapDevices = [{ device = swapDevice; }];
              boot.initrd.postDeviceCommands = postDeviceCommands;
            });

          simplesystem =
            { hostName
            , extraModules ? [ ]
            }: {
              inherit system;
              modules =
                [
                  ({ pkgs, lib, modulesPath, ... }:
                    {
                      imports =
                        [
                          (modulesPath + "/installer/scan/not-detected.nix")
                        ];

                      nix = {
                        extraOptions = "experimental-features = nix-command flakes";
                      };
                      # sound
                      sound.enable = true;
                      nixpkgs.config.pulseaudio = true;
                      nixpkgs.config.allowUnfree = true;
                      hardware.enableAllFirmware = true;
                      boot.loader.systemd-boot.enable = true;
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
                      services.xserver.enable = true;
                      services.xserver.desktopManager.gnome.enable = true;
                      services.xserver.displayManager.gdm.enable = true;
                      services.xserver.libinput.enable = true;
                      services.xserver.xkbOptions = "caps:none";
                      services.pcscd.enable = true;


                      # enable gpg
                      programs.gnupg.agent = {
                        enable = true;
                        pinentryFlavor = "curses";
                        enableSSHSupport = true;
                      };

                      programs.gnome-disks.enable = true;
                      environment.systemPackages = with pkgs; [
                        libreoffice
                        tor-browser-bundle-bin
                        docker-compose
                        firefox
                        git
                        gnome-text-editor
                        gnome.baobab
                        gnome.eog
                        gnome.file-roller
                        gnome.gnome-boxes
                        gnome.gnome-system-monitor
                        gnome.nautilus
                        gnome.gnome-power-manager
                        gnome-console
                        vlc
                        pinentry-curses
                        master.htop
                        tmux
                        lm_sensors
                        master.jetbrains.pycharm-community
                        smartmontools
                        master.jetbrains.goland
                        mendeley
                        nmap
                        silver-searcher
                        rnix-lsp
                        slack
                        master.helix
                        tig
                        xclip
                        nodejs
                        rustup
                        clang
                        julia-bin
                        rust-analyzer
                        gopls
                      ];
                      users.users.rxiao = {
                        isNormalUser = true;
                        extraGroups = [ "wheel" "docker" "vboxusers" ];
                      };
                      virtualisation.libvirtd.enable = true;
                      virtualisation.docker.enable = true;
                      virtualisation.docker.storageDriver = "zfs";
                      hardware.opengl.enable = true;
                      hardware.pulseaudio.enable = true;
                      systemd.enableUnifiedCgroupHierarchy = false;
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
                  services.tailscale.enable = true;
                  environment.interactiveShellInit = ''
                    alias athena='ssh rxiao@192.168.50.187'
                    alias artemis='ssh rxiao@artemis.silverpond.com.au'
                    export RUST_BACKTRACE=1
                  '';
                  environment.systemPackages = with pkgs; [ mongodb-compass vscode ];

                })
              intelCpuModule
            ];

          });
          # amd ryzen 7 1700
          athena = nixpkgs.lib.nixosSystem (simplesystem {
            hostName = "athena";
            extraModules = [
              ({ pkgs, lib, modulesPath, ... }:
                {
                  services.vscode-server.enable = true;
                  services.openssh = {
                    enable = true;
                    passwordAuthentication = false;
                  };
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
                  environment.systemPackages = with pkgs; [ bpytop nethogs qbittorrent-nox ];
                  systemd.services.qbittorrent-server = {
                    wantedBy = [ "multi-user.target" ];
                    description = "qbittorrent webserver";
                    serviceConfig = {
                      Type = "simple";
                      ExecStart = ''
                        /run/current-system/sw/bin/qbittorrent-nox --webui-port=8083                   
                      '';
                    };
                  };
                })
              (makeStorageModule {
                  postDeviceCommands = ''
                    zpool import -f data
                    zpool import -f torrents
                  '';
               })
              amdCpuModule
              vscode-server.nixosModule
              (makeNvidiaModule { powerlimit = 205; })
            ];
          });
          # amd ryzen 7 3700x
          wotan = nixpkgs.lib.nixosSystem (simplesystem {
            hostName = "wotan";
            extraModules = [
              amdCpuModule
              (makeNvidiaModule { powerlimit = 125; })
              (makeStorageModule {
                swapDevice = "/dev/disk/by-uuid/c99f9905-82ea-4431-a7ad-5a751deeb800";
                bootDevice = "/dev/disk/by-uuid/53D5-A050";
              })
            ];
          });
          # amd ryzen 3950x
          dante = nixpkgs.lib.nixosSystem (simplesystem {
            hostName = "dante";
            extraModules = [
              ({ pkgs, lib, modulesPath, ... }:
                {
                  services.tailscale.enable = true;
                  nix.settings = {
                    substituters = [ "https://hydra.iohk.io" "https://iohk.cachix.org" ];
                    trusted-public-keys = [ "hydra.iohk.io:f/Ea+s+dFdN+3Y/G+FDgSq+a5NEWhJGzdjvKNGv0/EQ=" "iohk.cachix.org-1:DpRUyj7h7V830dp/i6Nti+NEO2/nhblbov/8MW7Rqoo=" ];
                  };
                  environment.interactiveShellInit = ''
                    alias athena='ssh rxiao@192.168.50.187'
                    alias artemis='ssh rxiao@artemis.silverpond.com.au'
                    export RUST_BACKTRACE=1
                  '';
                  services.openssh = {
                    enable = true;
                    passwordAuthentication = true;
                  };
                  virtualisation.virtualbox.host.enable = true;
                  virtualisation.virtualbox.host.enableExtensionPack = true;
                  users.extraGroups.vboxusers.members = [ "user-with-access-to-virtualbox" ];
                  programs.steam = {
                    enable = true;
                    remotePlay.openFirewall = true; # Open ports in the firewall for Steam Remote Play
                    dedicatedServer.openFirewall = true; # Open ports in the firewall for Source Dedicated Server
                  };
                  environment.systemPackages = with pkgs; [ master.mongodb-compass master.vscode remmina ];
                })
              (makeStorageModule {
                  postDeviceCommands = ''
                    zpool import -f zdata
                    zpool import -f bigdisk
                  '';
              })
              amdCpuModule
              (makeNvidiaModule { powerlimit = 205; })
            ];
          });
        };
    };
}
