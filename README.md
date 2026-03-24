[English](README.md) | [Русский](README.ru_RU.md)

# RouteFlux

## Overview

RouteFlux is a lightweight OpenWrt-native Go application for managing Xray and V2Ray-compatible proxy subscriptions on routers and edge devices. It is designed for people who want an OpenWrt proxy manager for VLESS, VMess, Trojan, 3x-ui, Xray, router-based proxy routing, and subscription handling without hand-editing Xray JSON.

RouteFlux imports subscription URLs, raw `vless://`, `vmess://`, `trojan://`, and `ss://` links, plus valid 3x-ui or Xray JSON configs. It normalizes supported proxy outbounds into one node model, stores local state safely, and generates runtime configuration for Xray on OpenWrt and compatible forks such as ImmortalWrt.

## Features

- Import proxy subscriptions from a URL, raw share link, stdin, or valid 3x-ui/Xray JSON.
- Parse VLESS, VMess, Trojan, and Shadowsocks share links.
- Normalize supported 3x-ui/Xray proxy outbounds into RouteFlux nodes.
- Add, list, refresh, connect, disconnect, remove one subscription, or remove all subscriptions from the CLI or TUI.
- Select nodes manually or use automatic best-node selection with health checks and anti-flap logic.
- Generate Xray runtime config and reload the OpenWrt `init.d` service.
- Configure simple nftables-based routing for selected destination IPs, CIDRs, ranges, or LAN hosts.
- Manage DNS behavior through a dedicated `routeflux dns` command instead of mixing it into general settings.
- Start with a sensible DNS default profile: split DNS, DoH to Cloudflare, and local `.lan` names kept on local DNS.
- Persist subscriptions, settings, runtime state, and telemetry with atomic JSON writes.

## Quick Start

1. Install the RouteFlux binary on the router.
2. Add a subscription, share link, or valid 3x-ui/Xray JSON config.
3. Connect a node or enable auto mode.

See [Installation](#installation) and [Usage](#usage).

## Installation

1. Install Go `1.22` or later if you are building locally.
2. Use OpenWrt or ImmortalWrt with `nftables` available. OpenWrt `22.03+` is the practical baseline for the current firewall integration.
3. Build RouteFlux:

```bash
make build-openwrt
```

4. Copy the binary to the router. On many OpenWrt devices, `scp -O` works more reliably than default SFTP mode:

```bash
scp -O ./bin/openwrt/routeflux root@router:/usr/bin/routeflux
```

If your router does not provide SFTP support, stream the file over SSH:

```bash
ssh root@router 'cat > /tmp/routeflux.new && chmod 0755 /tmp/routeflux.new && mv /tmp/routeflux.new /usr/bin/routeflux' < ./bin/openwrt/routeflux
```

5. Optional: install the LuCI frontend files when you want the web interface.

Build a deploy bundle:

```bash
make package-openwrt
```

Copy it to the router and extract it at `/`:

```bash
scp -O ./dist/routeflux_0.1.0_all.tar.gz root@router:/tmp/
ssh root@router 'tar -xzf /tmp/routeflux_0.1.0_all.tar.gz -C / && rm -f /tmp/luci-indexcache && rm -rf /tmp/luci-modulecache && /etc/init.d/rpcd reload && /etc/init.d/uhttpd reload'
```

This installs:

- `/usr/bin/routeflux`
- `/usr/share/luci/menu.d/luci-app-routeflux.json`
- `/usr/share/rpcd/acl.d/luci-app-routeflux.json`
- `/www/luci-static/resources/view/routeflux/*.js`

6. Install Xray Core later when you want to use `connect`, `disconnect`, or generated runtime config. Ensure the service script exists at `/etc/init.d/xray`, or override it with `ROUTEFLUX_XRAY_SERVICE`.

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
routeflux remove --all
routeflux refresh --subscription sub-1234567890
routeflux refresh --all
routeflux connect --subscription sub-1234567890 --node abcdef123456
routeflux connect --auto --subscription sub-1234567890
routeflux disconnect
routeflux status
routeflux diagnostics
routeflux logs
routeflux daemon
routeflux daemon --once
routeflux settings get
routeflux settings set refresh-interval 1h
routeflux settings set auto-mode true
routeflux firewall get
routeflux firewall explain
routeflux firewall set targets 1.1.1.1 8.8.8.8/32
routeflux firewall set hosts 192.168.1.150
routeflux firewall set hosts 192.168.1.0/24
routeflux firewall set hosts 192.168.1.150-192.168.1.159
routeflux firewall set hosts all
routeflux firewall set block-quic true
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

Remove all stored subscriptions:

```bash
routeflux remove --all
```

Enable automatic best-node selection:

```bash
routeflux connect --auto --subscription sub-8b9f930214
```

Route all TCP traffic from one LAN device through RouteFlux:

```bash
routeflux firewall set hosts 192.168.1.150
routeflux connect --subscription sub-8b9f930214 --node 90c42d5dd302
```

Route a pool of LAN devices through RouteFlux:

```bash
routeflux firewall set hosts 192.168.1.32/27
routeflux connect --subscription sub-8b9f930214 --node 90c42d5dd302
```

Route all common private LAN clients through RouteFlux:

```bash
routeflux firewall set hosts all
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

Firewall workflow:

- Use `routeflux firewall get` to see current transparent routing settings.
- Use `routeflux firewall explain` to read plain-language explanations of host mode, target mode, and QUIC blocking.
- Use `routeflux firewall set hosts ...` to route selected LAN clients through RouteFlux.
- Use `routeflux firewall set targets ...` to route selected destination IPv4 addresses or ranges through RouteFlux.
- Use `routeflux firewall set port ...` and `routeflux firewall set block-quic ...` to tune the active firewall behavior.
- `routeflux firewall host ...` and `routeflux firewall status` still work as compatibility aliases.

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
- 3x-ui and Xray JSON imports, including JSON arrays of full configs, are normalized into RouteFlux nodes instead of being copied as full runtime configs.
- General settings and DNS settings are intentionally split. Use `routeflux settings` for app behavior and `routeflux dns` for runtime DNS.
- `routeflux firewall set hosts` accepts single IPv4 addresses, IPv4 CIDR pools, IPv4 ranges, and the aliases `all` or `*` for common private LAN ranges.
- `routeflux daemon` runs the background refresh loop. Use `--once` for a single scan or `--tick 30s` to override the scan interval.

## Architecture

The codebase is split into domain, parser, store, probe, backend, app, CLI, and TUI layers. See [docs/architecture.md](docs/architecture.md) for the full breakdown.

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
2. Copy the binary to the router. If you install manually instead of unpacking the release tarball, also copy `openwrt/root/etc/init.d/routeflux` to `/etc/init.d/routeflux` and make it executable.
3. Create `/etc/routeflux` if it does not exist.
4. Install Xray only when you want to connect traffic, and verify that `/etc/init.d/xray` can `reload`, `start`, and `stop`.
5. Enable the RouteFlux background refresh service when you want automatic subscription refresh:

```bash
/etc/init.d/routeflux enable
/etc/init.d/routeflux start
```

6. Run `routeflux add`, import a valid subscription or 3x-ui/Xray JSON config, and connect to a node.
7. If you need encrypted DNS, configure it through `routeflux dns` after the runtime is working.

## Limitations

- Xray is required for connect, disconnect, and runtime config generation.
- 3x-ui/Xray JSON import reads supported proxy outbounds only. It does not preserve full `dns`, `routing`, `inbounds`, outbound chaining, or other auxiliary runtime sections.
- The current Xray backend connects VLESS, VMess, and Trojan nodes. Shadowsocks parsing is available, but end-to-end Xray outbound generation for Shadowsocks is not wired yet.
- `dns.transport=dot` is defined in settings but is not applied by the current Xray backend.
- Transparent router traffic interception is not fully automated in MVP.
- Automatic subscription refresh requires the RouteFlux daemon or OpenWrt `/etc/init.d/routeflux` service to be running.
- Simple firewall routing currently supports destination IPv4 targets, source IPv4 hosts, CIDR pools, IPv4 ranges, and the `all` or `*` LAN-wide shortcut. QUIC blocking is host-mode only.
- A LuCI MVP lives in `luci-app-routeflux` with `Overview`, `Subscriptions`, `Firewall`, `DNS`, `Settings`, `Diagnostics`, and `Logs` pages.

## License

MIT
