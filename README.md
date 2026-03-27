[English](README.md) | [Русский (скоро будет)](README.ru_RU.md)

# RouteFlux

## Overview

RouteFlux is a lightweight OpenWrt-native Go application for managing Xray and V2Ray-compatible proxy subscriptions on routers and edge devices. It is designed for people who want an OpenWrt proxy manager for VLESS, VMess, Trojan, 3x-ui, Xray, router-based proxy routing, and subscription handling without hand-editing Xray JSON.

RouteFlux imports subscription URLs, raw `vless://`, `vmess://`, and `trojan://` links, plus valid 3x-ui or Xray JSON configs. It normalizes supported proxy outbounds into one node model, stores local state safely, and generates runtime configuration for Xray on OpenWrt and compatible forks such as ImmortalWrt.

RouteFlux ships one runtime with three operator surfaces: CLI, LuCI, and TUI. On OpenWrt, the primary day-to-day surfaces are the CLI and LuCI web interface, while the TUI remains a keyboard-first local interface for SSH sessions and terminal-driven workflows.

## Features

- Import proxy subscriptions from a URL, raw share link, stdin, or valid 3x-ui/Xray JSON.
- Parse VLESS, VMess, and Trojan share links.
- Normalize supported 3x-ui/Xray proxy outbounds into RouteFlux nodes.
- Add, list, refresh, connect, disconnect, remove one subscription, or remove all subscriptions from the CLI or TUI.
- Select nodes manually or use automatic best-node selection with daemon-backed health checks, live failover, and anti-flap logic.
- Validate candidate Xray configs with `xray -test`, keep a last-known-good backup, and safely reload the OpenWrt `init.d` service.
- Configure simple nftables-based routing for selected destination IPs, CIDRs, ranges, or LAN hosts.
- Manage DNS behavior through a dedicated `routeflux dns` command instead of mixing it into general settings.
- Start with a sensible DNS default profile: split DNS, DoH to Cloudflare, and local `.lan` names kept on local DNS.
- Persist subscriptions, settings, runtime state, and telemetry with atomic JSON writes.
- Restore the last active runtime selection during daemon startup after reboot when persisted state is valid.

## Quick Start

Install the current beta release from your computer. Set `ROUTEFLUX_TAG` to the release tag you want to install:

```bash
ROUTEFLUX_TAG=v0.1.3-beta.10
wget -O /tmp/routeflux-install.sh "https://github.com/Alaxay8/routeflux/releases/download/${ROUTEFLUX_TAG}/install.sh" && sh /tmp/routeflux-install.sh
```

GitHub does not serve prerelease assets from `releases/latest/download`. While RouteFlux is still published as a beta prerelease, use a tag-pinned release URL.
If Xray is missing, the installer automatically downloads and installs the bundled Xray runtime for the matching release and architecture before installing RouteFlux.

Current easy-install release assets are provided for:

- `mipsel_24kc`
- `x86_64`

To remove RouteFlux, the bundled Xray runtime, and LuCI assets from the router, use the same `ROUTEFLUX_TAG` value that you used for installation:

```bash
wget -O /tmp/routeflux-uninstall.sh "https://github.com/Alaxay8/routeflux/releases/download/${ROUTEFLUX_TAG}/uninstall.sh" && sh /tmp/routeflux-uninstall.sh
```

After install:

1. Open LuCI at `Services -> RouteFlux`, or use the CLI over SSH.
2. Add a subscription, share link, or valid 3x-ui/Xray JSON config.
3. Add a subscription and connect a node or enable auto mode.

See [Installation](#installation) and [Usage](#usage).

## Installation

1. Fastest path: use the installer from the current beta GitHub release:

```bash
ROUTEFLUX_TAG=v0.1.3-beta.10
wget -O /tmp/routeflux-install.sh "https://github.com/Alaxay8/routeflux/releases/download/${ROUTEFLUX_TAG}/install.sh" && sh /tmp/routeflux-install.sh
```

If you publish a non-prerelease stable release later, you can switch this command back to `releases/latest/download/install.sh`.
The installer will auto-install the bundled Xray runtime when `/usr/bin/xray` or `/etc/init.d/xray` is missing.

1. For local builds, install Go `1.22` or later.
2. Use OpenWrt or ImmortalWrt with `nftables` available. OpenWrt `22.03+` is the practical baseline for the current firewall integration.
3. Build RouteFlux from source:

```bash
make build-openwrt
```

Build the x86_64 OpenWrt test binary used by the QEMU integration suite:

```bash
make build-openwrt-x86_64
```

1. Copy the binary to the router. On many OpenWrt devices, `scp -O` works more reliably than default SFTP mode:

```bash
scp -O ./bin/openwrt/routeflux root@router:/usr/bin/routeflux
```

If your router does not provide SFTP support, stream the file over SSH:

```bash
cat > /tmp/routeflux.new && chmod 0755 /tmp/routeflux.new && mv /tmp/routeflux.new /usr/bin/routeflux < ./bin/openwrt/routeflux
```

1. Optional: install the LuCI frontend files when you want the web interface.

Build OpenWrt deployment artifacts:

```bash
make package-openwrt
```

This creates:

```bash
VERSION="$(git describe --tags --always --dirty | sed 's/^v//')"
```

- `dist/routeflux_${VERSION}_mipsel_24kc.ipk`
- `dist/routeflux_${VERSION}_mipsel_24kc.tar.gz`
- `dist/xray_${VERSION}_mipsel_24kc.tar.gz`

Build the release bundle used by GitHub Releases:

```bash
make package-release
```

This creates:

- `dist/routeflux_${VERSION}_mipsel_24kc.ipk`
- `dist/routeflux_${VERSION}_mipsel_24kc.tar.gz`
- `dist/routeflux_${VERSION}_x86_64.ipk`
- `dist/routeflux_${VERSION}_x86_64.tar.gz`
- `dist/xray_${VERSION}_mipsel_24kc.tar.gz`
- `dist/xray_${VERSION}_x86_64.tar.gz`
- `dist/install.sh`
- `dist/uninstall.sh`

For a reliable manual install, copy the tarball to the router and extract it at `/`:

```bash
VERSION="$(git describe --tags --always --dirty | sed 's/^v//')"
scp -O "./dist/routeflux_${VERSION}_mipsel_24kc.tar.gz" root@router:/tmp/
ssh root@router "tar -xzf /tmp/routeflux_${VERSION}_mipsel_24kc.tar.gz -C / && rm -f /tmp/luci-indexcache && rm -rf /tmp/luci-modulecache && /etc/init.d/rpcd reload && /etc/init.d/uhttpd reload"
```

This installs:

- `/usr/bin/routeflux`
- `/usr/share/luci/menu.d/luci-app-routeflux.json`
- `/usr/share/rpcd/acl.d/luci-app-routeflux.json`
- `/www/luci-static/resources/view/routeflux/*.js`

To make `opkg install routeflux` work by package name, build the package with the OpenWrt SDK or buildroot, publish it in an `opkg` feed with `Packages.gz`, add that feed to the router, then run `opkg update` and `opkg install routeflux`.

1. The GitHub release installer will auto-install the bundled Xray runtime when needed. For custom setups, you can still override the Xray binary and service paths with `ROUTEFLUX_XRAY_BINARY` and `ROUTEFLUX_XRAY_SERVICE`.

## Usage

LuCI examples:

### Overview dashboard

![RouteFlux LuCI overview](pic/overview.png)

Full-size image: [overview.png](pic/overview.png)

The `Overview` page is the main LuCI dashboard for day-to-day RouteFlux control. It gives a quick summary of the current connection state, the active profile, and the most common actions without switching to the CLI.

Dashboard elements:

- `State`: shows whether RouteFlux is currently connected and highlights the primary runtime status.
- `Mode`: shows how the current connection was selected, for example `manual` or automatic mode.
- `Provider`: shows the provider group for the active subscription.
- `Profile`: shows the selected imported profile inside that provider group.
- `Node`: shows the active proxy node. When possible, LuCI presents a human-friendly location instead of a raw node label.
- `DNS`: shows the active DNS mode, for example `split` when encrypted upstream DNS is used for external domains and local names stay on the router DNS.
- `Firewall`: shows how RouteFlux routing rules are applied. Typical values are `disabled`, `hosts`, or `targets`.
- `Last Refresh`: shows when the active subscription was last updated.

The `Actions` block is used for the most common control flow:

- `Subscription`: selects which imported subscription or profile the quick actions should use.
- `Connect Auto`: enables automatic best-node selection for the selected subscription. Continuous health monitoring and live failover require the daemon to be running.
- `Refresh Active`: refreshes the currently active subscription and reloads the dashboard state.
- `Disconnect`: stops the current RouteFlux connection without removing saved subscriptions.

The `Subscriptions` table gives a compact inventory of imported profiles:

- `Name`: provider and profile label shown in LuCI.
- `Nodes`: number of parsed proxy nodes available in that profile.
- `Updated`: last successful refresh time for that subscription.
- `Status`: parser result for the latest import or refresh operation.

If the active subscription fails to refresh or connect, LuCI also shows a `Last Error` panel under the table so the failure reason is visible without opening logs first.

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
- `ROUTEFLUX_XRAY_BINARY`: override the Xray binary used for `xray -test`.
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

## Development

Build and test locally:

```bash
make fmt
make lint
make coverage-runtime
make build
```

Cross-build for OpenWrt:

```bash
make build-openwrt
make build-openwrt-x86_64
```

Run the OpenWrt/QEMU integration suite:

```bash
make test-integration
```

OpenWrt package filenames use the current Git tag or `git describe` version without the leading `v`.

Project notes:

- The parser, selector, firewall integration, DNS rendering, and Xray config generation are covered by unit tests and golden files.
- `routeflux add` accepts URLs, raw share links, or stdin, so it works well with copy-paste and shell pipelines.
- 3x-ui and Xray JSON imports, including JSON arrays of full configs, are normalized into RouteFlux nodes instead of being copied as full runtime configs.
- General settings and DNS settings are intentionally split. Use `routeflux settings` for app behavior and `routeflux dns` for runtime DNS.
- `routeflux firewall set hosts` accepts single IPv4 addresses, IPv4 CIDR pools, IPv4 ranges, and the aliases `all` or `*` for common private LAN ranges.
- `routeflux daemon` runs the background refresh loop and auto-mode health monitor. Use `--once` for a single scan or `--tick 30s` to override the refresh scan interval.

## Architecture

The codebase is split into domain, parser, store, probe, backend, app, CLI, and TUI layers. See [docs/architecture.md](docs/architecture.md) for the full breakdown.

## Supported Protocols

- VLESS
- VMess
- Trojan

## TUI

The TUI is a keyboard-driven interface that focuses on provider-first navigation:

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
2. Copy the binary to the router. If you install manually instead of installing the `.ipk`, also copy `openwrt/root/etc/init.d/routeflux` to `/etc/init.d/routeflux` and make it executable.
3. Create `/etc/routeflux` if it does not exist.
4. Install Xray only when you want to connect traffic, and verify that `/etc/init.d/xray` can `reload`, `start`, and `stop`.
5. Enable the RouteFlux background refresh service when you want automatic subscription refresh and reboot-time runtime restore:

```bash
/etc/init.d/routeflux enable
/etc/init.d/routeflux start
```

1. Run `routeflux add`, import a valid subscription or 3x-ui/Xray JSON config, and connect to a node.
2. If you need encrypted DNS, configure it through `routeflux dns` after the runtime is working.

## Limitations

- Xray is required for connect, disconnect, and runtime config generation.
- 3x-ui/Xray JSON import reads supported proxy outbounds only. It does not preserve full `dns`, `routing`, `inbounds`, outbound chaining, or other auxiliary runtime sections.
- The current Xray backend connects VLESS, VMess, and Trojan nodes.
- Transparent router traffic interception still requires explicit firewall configuration for the traffic you want to route through RouteFlux.
- Automatic subscription refresh, continuous auto-mode health checks, and live failover require the RouteFlux daemon or OpenWrt `/etc/init.d/routeflux` service to be running.
- Simple firewall routing currently supports destination IPv4 targets, source IPv4 hosts, CIDR pools, IPv4 ranges, and the `all` or `*` LAN-wide shortcut. QUIC blocking is host-mode only.
- CLI, LuCI, and TUI all operate on the same persisted state and runtime backend.
- The LuCI frontend lives in `luci-app-routeflux` with `Overview`, `Subscriptions`, `Firewall`, `DNS`, `Settings`, `Diagnostics`, and `Logs` pages.

## Support Matrix

- Primary operator surfaces: CLI and LuCI on OpenWrt.
- Additional operator surface: TUI for keyboard-first local workflows.
- Supported router family: OpenWrt and compatible forks such as ImmortalWrt.
- Practical runtime baseline: OpenWrt `22.03+` with `nftables` available.
- Runtime dependency: Xray service script available at `/etc/init.d/xray` unless overridden.
- Firewall scope: IPv4 targets, IPv4 source hosts, IPv4 CIDRs, IPv4 ranges, and the `all` or `*` LAN shortcut.
- DNS scope: `plain` and `doh` are supported today; `dot` remains reserved for a future backend.

## Upgrade Policy

- In-place upgrades must preserve `/etc/routeflux`.
- Settings and state files with missing or older schema versions are upgraded on load to the current schema.
- Malformed `settings.json` or `state.json` is backed up to `*.corrupt-<UTC>.json`, then replaced with a fresh canonical file so startup can continue safely.
- Future schema versions are rejected and require a newer RouteFlux build or a manual migration path.
- OpenWrt package and tarball versions are derived from the current Git tag or `git describe` output without the leading `v`.

## License

MIT
