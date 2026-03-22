[English](README.md) | [العربية](README.ar.md) | [فارسی](README.fa.md) | [中文](README.zh_CN.md) | [Español](README.es.md) | [Русский](README.ru_RU.md)

# RouteFlux

## Overview

RouteFlux is a lightweight OpenWrt-native Go application for managing Xray and V2Ray-compatible proxy subscriptions on routers and edge devices. It is designed for people who want an OpenWrt proxy manager for VLESS, VMess, Trojan, 3x-ui, Xray, router-based proxy routing, and subscription handling without hand-editing Xray JSON.

RouteFlux imports subscription URLs, raw `vless://`, `vmess://`, `trojan://`, and `ss://` links, plus valid 3x-ui or Xray JSON configs. It normalizes supported proxy outbounds into one node model, stores local state safely, and generates runtime configuration for Xray on OpenWrt and compatible forks such as ImmortalWrt.

## Features

- Import proxy subscriptions from a URL, raw share link, stdin, or valid 3x-ui/Xray JSON.
- Parse VLESS, VMess, Trojan, and Shadowsocks share links.
- Normalize supported 3x-ui/Xray proxy outbounds into RouteFlux nodes.
- Add, list, refresh, connect, disconnect, and remove subscriptions from the CLI or TUI.
- Select nodes manually or use automatic best-node selection with health checks and anti-flap logic.
- Generate Xray runtime config and reload the OpenWrt `init.d` service.
- Configure simple nftables-based routing for selected destination IPs, CIDRs, ranges, or LAN hosts.
- Manage DNS behavior through a dedicated `routeflux dns` command instead of mixing it into general settings.
- Start with a sensible DNS default profile: split DNS, DoH to Cloudflare, and local `.lan` names kept on local DNS.
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

1. Copy the binary to the router. On many OpenWrt devices, `scp -O` works more reliably than default SFTP mode:

```bash
scp -O ./bin/openwrt/routeflux root@router:/usr/bin/routeflux
```

If your router does not provide SFTP support, stream the file over SSH:

```bash
ssh root@router 'cat > /tmp/routeflux.new && chmod 0755 /tmp/routeflux.new && mv /tmp/routeflux.new /usr/bin/routeflux' < ./bin/openwrt/routeflux
```

1. Ensure the Xray service script exists at `/etc/init.d/xray`, or override it with `ROUTEFLUX_XRAY_SERVICE`.

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
routeflux dns get
routeflux dns explain
routeflux dns set default
routeflux dns set mode split
routeflux dns set transport doh
routeflux dns set servers "dns.google,1.1.1.1"
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

Use encrypted DNS for external domains but keep home-network names local:

```bash
routeflux dns set default
```

This applies the current RouteFlux recommended profile:

```text
mode=split
transport=doh
servers=1.1.1.1,1.0.0.1
bootstrap=
direct-domains=domain:lan,full:router.lan
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

DNS workflow:

- RouteFlux starts with a practical default profile: `split + doh + 1.1.1.1/1.0.0.1 + domain:lan,full:router.lan`.
- Use `routeflux dns get` to see current DNS settings.
- Use `routeflux dns explain` to read plain-language explanations of each DNS mode and transport.
- Use `routeflux dns set default` to restore the recommended everyday DNS profile in one command.
- Use `routeflux dns set ...` to change DNS behavior.
- Keep using `routeflux settings` for general app settings such as refresh interval, auto mode, and log level.

DNS modes:

- `system`: RouteFlux leaves DNS alone and does not write a DNS block into the Xray config.
- `remote`: all DNS requests go to the DNS servers you selected.
- `split`: local home-network names stay local, while the rest goes to your selected DNS servers.
- `disabled`: RouteFlux skips writing a DNS block. This is mainly useful when you want to be explicit.

DNS transports:

- `plain`: regular DNS, not encrypted.
- `doh`: DNS over HTTPS. This is the working encrypted DNS option in the current backend.
- `dot`: DNS over TLS. The setting exists, but the current Xray backend does not apply it yet.

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

- The parser, selector, firewall integration, DNS rendering, and Xray config generation are covered by unit tests and golden files.
- `routeflux add` accepts URLs, raw share links, or stdin, so it works well with copy-paste and shell pipelines.
- 3x-ui and Xray JSON imports are normalized into RouteFlux nodes instead of being copied as full runtime configs.
- General settings and DNS settings are intentionally split. Use `routeflux settings` for app behavior and `routeflux dns` for runtime DNS.

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
6. If you need encrypted DNS, configure it through `routeflux dns` after the runtime is working.

## Limitations

- Xray is required for connect, disconnect, and runtime config generation.
- 3x-ui/Xray JSON import reads supported proxy outbounds only. It does not preserve full `dns`, `routing`, `inbounds`, outbound chaining, or other auxiliary runtime sections.
- The current Xray backend connects VLESS, VMess, and Trojan nodes. Shadowsocks parsing is available, but end-to-end Xray outbound generation for Shadowsocks is not wired yet.
- `dns.transport=dot` is defined in settings but is not applied by the current Xray backend.
- Transparent router traffic interception is not fully automated in MVP.
- Simple firewall routing currently supports IPv4 targets, IPv4 ranges, LAN hosts, and TCP redirect. QUIC blocking is host-mode only.
- There is no LuCI web UI or native package feed integration yet.

## License

MIT
