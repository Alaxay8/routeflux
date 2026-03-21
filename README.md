# RouteFlux

## Overview
RouteFlux is a lightweight OpenWrt-native Go application for managing Xray and V2Ray-compatible proxy subscriptions on routers and edge devices. It is built for users looking for an OpenWrt proxy manager for VLESS, VMess, Trojan, 3x-ui, Xray, and router-based proxy workflows without hand-editing Xray JSON.

RouteFlux imports subscription URLs, raw `vless://`, `vmess://`, `trojan://`, and `ss://` links, plus valid 3x-ui or Xray JSON configs. It normalizes supported proxy outbounds into one node model, stores state locally, and generates runtime configuration for Xray on OpenWrt and compatible forks such as ImmortalWrt.

## Features
- Import proxy subscriptions from URL, raw share link, stdin, or valid 3x-ui/Xray JSON.
- Parse VLESS, VMess, Trojan, and Shadowsocks share links.
- Normalize supported proxy outbounds from 3x-ui/Xray JSON into RouteFlux nodes.
- Add, list, refresh, connect, disconnect, and remove subscriptions from the CLI or TUI.
- Select nodes manually or use automatic best-node selection with health checks and anti-flap logic.
- Generate Xray runtime config and reload the OpenWrt `init.d` service.
- Configure simple nftables-based routing for selected destination IPs, CIDRs, ranges, or LAN hosts.
- Persist subscriptions, settings, runtime state, and telemetry with atomic JSON writes.

## Quick Start
```bash
make build
./bin/routeflux add 'vless://uuid@example.com:443?...#Example'
./bin/routeflux list subscriptions
./bin/routeflux connect --subscription sub-1234567890 --node abcdef123456
```

## Installation
1. Install Go `1.22` or later if you are building locally.
2. Install Xray Core on the target router.
3. Use OpenWrt or ImmortalWrt with `nftables` available. OpenWrt `22.03+` is the practical baseline for the current firewall integration.
4. Build RouteFlux:
```bash
make build-openwrt
```
5. Copy the binary to the router:
```bash
scp ./bin/openwrt/routeflux root@router:/usr/bin/routeflux
```
6. Ensure the Xray service script exists at `/etc/init.d/xray`, or override it with `ROUTEFLUX_XRAY_SERVICE`.

## Usage
CLI examples:
```bash
routeflux add
routeflux add https://provider.example/subscription
routeflux add 'vless://uuid@example.com:443?...#Example'
routeflux add < 3x-ui-config.json
routeflux list subscriptions
routeflux list nodes --subscription sub-1234567890
routeflux remove sub-1234567890
routeflux refresh --subscription sub-1234567890
routeflux refresh --all
routeflux connect --subscription sub-1234567890 --node abcdef123456
routeflux connect --auto --subscription sub-1234567890
routeflux disconnect
routeflux status
routeflux settings get
routeflux settings set refresh-interval 1h
routeflux settings set auto-mode true
routeflux firewall set 1.1.1.1 8.8.8.8/32
routeflux firewall host 192.168.1.150
routeflux firewall status
routeflux tui
```

## Examples
Import a valid 3x-ui or Xray JSON config:
```bash
routeflux add < ./client-config.json
```

Remove a stored subscription:
```bash
routeflux remove sub-8b9f930214
```

Enable automatic best-node selection:
```bash
routeflux connect --auto --subscription sub-8b9f930214
```

Route all TCP traffic from a LAN host through RouteFlux:
```bash
routeflux firewall host 192.168.1.150
routeflux connect --subscription sub-8b9f930214 --node 90c42d5dd302
```

## Configuration
RouteFlux stores state under `/etc/routeflux` on OpenWrt by default. For local development it uses `./.routeflux`.

Important environment variables:
- `ROUTEFLUX_ROOT`: override the state directory.
- `ROUTEFLUX_XRAY_CONFIG`: override the generated Xray config path.
- `ROUTEFLUX_XRAY_SERVICE`: override the Xray service control script.
- `ROUTEFLUX_FIREWALL_RULES`: override the generated nftables rules file path.

Persisted files:
- `/etc/routeflux/subscriptions.json`
- `/etc/routeflux/settings.json`
- `/etc/routeflux/state.json`

## Development
Build and test locally:
```bash
make fmt
make test
make coverage
make build
```

Cross-build for OpenWrt:
```bash
make build-openwrt
```

Project notes:
- The parser, selector, firewall integration, and Xray config generation are covered by unit tests and golden files.
- `routeflux add` accepts URLs, raw share links, or stdin so it works well with copy-paste and shell pipelines.
- 3x-ui and Xray JSON imports are normalized into RouteFlux nodes instead of being copied as full runtime configs.

## Architecture
The codebase is split into domain, parser, store, probe, backend, app, CLI, and TUI layers. See [docs/architecture.md](/Users/alexey/dev/routeflux/docs/architecture.md) for the full breakdown.

## Supported Protocols
- VLESS
- VMess
- Trojan
- Shadowsocks share-link parsing

## TUI
The MVP TUI is keyboard-driven and focuses on provider-first navigation:
- `j` / `k`: move between VPN services
- `h` / `l`: move between profiles inside the selected service
- `n` / `p`: move between locations inside the selected profile
- `c`: connect selected node
- `a`: enable auto selection on the selected subscription
- `r`: refresh selected subscription
- `s`: open settings
- `d`: disconnect
- `q`: quit

Placeholder screenshots:
- `docs/images/tui-main.txt`
- `docs/images/tui-settings.txt`

## OpenWrt Deployment
1. Build with `make build-openwrt`.
2. Copy the binary to the router.
3. Create `/etc/routeflux` if it does not exist.
4. Install Xray and verify that `/etc/init.d/xray` can `reload`, `start`, and `stop`.
5. Run `routeflux add`, import a valid subscription or 3x-ui/Xray JSON config, and connect to a node.

## Limitations
- Xray is required for connect, disconnect, and runtime config generation.
- 3x-ui/Xray JSON import reads supported proxy outbounds only. It does not preserve full `dns`, `routing`, `inbounds`, outbound chaining, or other auxiliary runtime sections.
- The current Xray backend connects VLESS, VMess, and Trojan nodes. Shadowsocks parsing is available, but end-to-end Xray outbound generation for Shadowsocks is not wired yet.
- Transparent router traffic interception is not fully automated in MVP.
- Simple firewall routing currently supports IPv4 targets, IPv4 ranges, LAN hosts, and TCP redirect. QUIC blocking is host-mode only.
- There is no LuCI web UI or native package feed integration yet.

## License
MIT
