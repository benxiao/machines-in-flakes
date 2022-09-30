{
  description = "my computers in flakes";
  inputs.nixpkgs.url = "github:nixos/nixpkgs/nixos-22.05";
  inputs.nixpkgs-master.url = "github:nixos/nixpkgs";
  inputs.vscode-server.url = "github:nixos/nixpkgs";
  outputs = { self, nixpkgs, nixpkgs-master }:
    let
      overlay-master = final: prev: {
        master = nixpkgs-master.legacyPackages.${prev.system};
      
      };
    in
    {
      nixosConfigurations =
        let
          simplesystem =
            { hostName
            , hardware_configurations
            , rootPool ? "zroot/root"
            , bootDevice ? "/dev/nvme0n1p3"
            , swapDevice ? "/dev/nvme0n1p2"
            }: {
              system = "x86_64-linux";
              modules = [
                vscode-server.nixosModule
                ({ config, pkgs, ... }: { nixpkgs.overlays = [ overlay-master ]; })

                ({ pkgs, lib, modulesPath, ... }:
                  {
                    imports =
                      [
                        (modulesPath + "/installer/scan/not-detected.nix")
                      ];

                    boot.initrd.availableKernelModules = [ "nvme" ];
                    fileSystems."/" = { device = rootPool; fsType = "zfs"; };
                    fileSystems."/boot" = { device = bootDevice; fsType = "vfat"; };
                    swapDevices = [{ device = swapDevice; }];
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
                    networking.networkmanager = {
                      enable = true;
                      wifi.powersave = false;
                    };
                    time.timeZone = "Australia/Melbourne";

                    services.logind.extraConfig = ''
                      RuntimeDirectorySize=10G
                    '';

                    i18n.defaultLocale = "en_AU.UTF-8";
                    services.gnome.core-utilities.enable = false;
                    services.gnome.tracker-miners.enable = false;
                    services.gnome.tracker.enable = false;
                    services.xserver.desktopManager.gnome.enable = true;
                    services.xserver.displayManager.gdm.enable = true;
                    services.xserver.enable = true;
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
                      qbittorrent-nox
                      tor-browser-bundle-bin
                      awscli2
                      awsebcli
                      docker-compose
                      evince
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
                      vlc
                      pinentry-curses
                      htop
                      tmux
                      lm_sensors
                      jetbrains.pycharm-community
                      smartmontools
                      jetbrains.goland
                      mendeley
                      nmap
                      obs-studio
                      silver-searcher
                      rnix-lsp
                      slack
                      master.helix
                      tig
                      xclip
                      chromium
                      nodejs
                      rustup
                      julia-bin
                      clang
                      julia-bin
                      rust-analyzer
                      gopls
                      gnome-console
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
                hardware_configurations

              ];
            };
        in
        {
          # Lenovo T490
          apollo = nixpkgs.lib.nixosSystem (simplesystem {
            hostName = "apollo";
            hardware_configurations = ({ pkgs, lib, modulesPath, ... }:
              {
                services.tailscale.enable = true;
                powerManagement.cpuFreqGovernor = "powersave";
                hardware.cpu.intel.updateMicrocode = true;
                environment.interactiveShellInit = ''
                  alias athena='ssh rxiao@192.168.50.187'
                  alias artemis='ssh rxiao@artemis.silverpond.com.au'
                  export RUST_BACKTRACE=1
                '';

                environment.systemPackages = with pkgs; [ mongodb-compass vscode ];
                virtualisation.virtualbox.host.enable = true;
                virtualisation.virtualbox.host.enableExtensionPack = true;
                users.extraGroups.vboxusers.members = [ "user-with-access-to-virtualbox" ];

              });

          });
          # amd ryzen 7 1700
          athena = nixpkgs.lib.nixosSystem (simplesystem {
            hostName = "athena";
            hardware_configurations = ({ pkgs, lib, modulesPath, ... }:
              {
                services.vscode-server.enable = true;
                services.xserver.videoDrivers = [ "nvidia" ];
                virtualisation.docker.enableNvidia = true;
                hardware.opengl.driSupport32Bit = true;
                hardware.cpu.amd.updateMicrocode = true;
                boot.kernelModules = [ "kvm-amd" ];
                services.openssh = {
                  enable = true;
                  #passwordAuthentication = true;
                };
                services.xserver.displayManager.gdm.autoSuspend = true;
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
                hardware.nvidia.nvidiaPersistenced = true;
                boot.initrd.postDeviceCommands = ''
                  zpool import -f data
                  zpool import -f torrents
                '';
                environment.systemPackages = with pkgs; [ bpytop nethogs ];
                systemd.services.nvidia-power-limiter = {
                  wantedBy = [ "multi-user.target" ];
                  description = "set power limit for nvidia gpus";
                  serviceConfig = {
                    Type = "simple";
                    ExecStart = ''
                      /run/current-system/sw/bin/bash -c "/run/current-system/sw/bin/nvidia-smi -i 0 -pl 205 &&  /run/current-system/sw/bin/nvidia-smi -i 1 -pl 75" 
                    '';
                  };
                };
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
              });
          });
          # amd ryzen 7 3700x
          wotan = nixpkgs.lib.nixosSystem (simplesystem {
            hostName = "wotan";
            swapDevice = "/dev/disk/by-uuid/79ef359f-1882-4427-a93e-363259bc2445";
            bootDevice = "/dev/disk/by-uuid/07D2-41D4";
            hardware_configurations = ({ pkgs, lib, modulesPath, ... }: { });
          });
          # amd ryzen 3950x
          dante = nixpkgs.lib.nixosSystem (simplesystem {
            hostName = "dante";
            hardware_configurations = ({ pkgs, lib, modulesPath, ... }:
              {
                services.xserver.videoDrivers = [ "nvidia" ];
                virtualisation.docker.enableNvidia = true;
                hardware.opengl.driSupport32Bit = true;
                services.tailscale.enable = true;
                hardware.cpu.amd.updateMicrocode = true;
                nix.settings = {
                  substituters = [ "https://hydra.iohk.io" "https://iohk.cachix.org" ];
                  trusted-public-keys = [ "hydra.iohk.io:f/Ea+s+dFdN+3Y/G+FDgSq+a5NEWhJGzdjvKNGv0/EQ=" "iohk.cachix.org-1:DpRUyj7h7V830dp/i6Nti+NEO2/nhblbov/8MW7Rqoo=" ];
                };
                boot.kernelModules = [ "kvm-amd" ];
                hardware.nvidia.nvidiaPersistenced = true;
                environment.interactiveShellInit = ''
                  alias athena='ssh rxiao@192.168.50.187'
                  alias artemis='ssh rxiao@artemis.silverpond.com.au'
                  export RUST_BACKTRACE=1
                '';
                boot.initrd.postDeviceCommands = ''
                  zpool import -f zdata
                  zpool import -f bigdisk
                '';
                systemd.services.nvidia-power-limiter = {
                  wantedBy = [ "multi-user.target" ];
                  description = "set power limit for nvidia gpus";
                  serviceConfig = {
                    Type = "simple";
                    ExecStart = ''
                      /run/current-system/sw/bin/nvidia-smi -pl 205
                    '';
                  };
                };
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
                environment.systemPackages = with pkgs; [ mongodb-compass ];
              });
          });
        };
    };
}
