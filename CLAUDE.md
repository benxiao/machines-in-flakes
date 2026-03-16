# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This is a NixOS flake repository managing multiple machines. All machine configurations live in `flake.nix` as a single file.

## Common Commands

```bash
# Rebuild and switch the current machine
sudo nixos-rebuild switch --flake .#<hostname>

# Build without switching (dry run)
sudo nixos-rebuild build --flake .#<hostname>

# Update flake inputs
nix flake update

# Update a specific input
nix flake update nixpkgs

# Check flake evaluation
nix flake check

# Format nix files
nixpkgs-fmt flake.nix
```

## Architecture

Everything is in `flake.nix`. The structure uses reusable modules defined as local `let` bindings:

- **`makeSystemModule`** — base module applied to all machines (GNOME, ZFS, Docker, Tailscale, common packages, user `rxiao`)
- **`makeStorageModule`** — ZFS boot/root/swap configuration per machine
- **`makeServerModule`** — disables auto-suspend, enables SSH
- **`makePython3Module`** — Python environment with optional extra packages (uses `nixpkgs-unstable`)
- **`intelCpuModule` / `amdCpuModule`** — CPU-specific microcode and kernel modules
- **`nvidiaModule`** — NVIDIA drivers with container toolkit
- **`desktopAppsModule`** — GUI apps (Chrome, VSCode, Slack, etc.)
- **`googleSDKPackageModule`** — Google Cloud SDK with GKE auth and PubSub emulator
- **`printerModule`** — CUPS + Avahi for network printing
- **`checkRouterAliveModule`** — systemd service that shuts down if router is unreachable

## Machines

| Hostname | Hardware | Notes |
|----------|----------|-------|
| `apollo` | Lenovo ThinkPad T490 (Intel) | Laptop with desktop apps |
| `athena` | AMD Ryzen 7 3700X | Home server, NVIDIA, VSCode Server, extra ZFS pools |
| `wotan` | Intel i5-13500 | Desktop with NVIDIA |
| `dante` | AMD Ryzen 3950X | Desktop with NVIDIA, Steam, Temporal, PostgreSQL |

## nixpkgs Channels

- `nixpkgs` → `nixos-25.11` (stable, default)
- `nixpkgs-legacy` → `nixos-25.05`
- `nixpkgs-unstable` → `nixpkgs-unstable` (used for Python env and some packages like `ffmpeg-full`, `claude-code`)
- `nixpkgs-master` → `master` (used for `helix`)

## Key Conventions

- `nixpkgs.config.allowUnfree = true` is set globally
- Docker uses ZFS storage driver; service waits for ZFS import
- All machines use ZFS root with systemd-boot (EFI)
- Default editor: `helix` (`hx`)
- `networking.firewall.enable = false` on all machines
- `system.stateVersion = "25.05"`
