[English](README.md) | [Русский](README.ru_RU.md)

# RouteFlux

RouteFlux is an OpenWrt-native Xray subscription manager for routers and edge devices.

It helps you import proxy subscriptions, pick the right node, apply router traffic rules, and manage DNS without hand-editing Xray JSON. If RouteFlux saves you time, consider starring the repository and sharing it with other OpenWrt users.

## Overview

RouteFlux is built for people who want a practical proxy workflow on OpenWrt:

- Import a subscription URL, raw `vless://`, `vmess://`, or `trojan://` link, or a valid 3x-ui/Xray JSON config.
- Connect manually to a specific node or let RouteFlux pick the best node automatically.
- Manage everything from the CLI, LuCI web UI, or the local TUI.
- Keep router routing and DNS settings readable instead of burying them inside generated Xray config.

The current runtime target is OpenWrt and compatible forks such as ImmortalWrt. OpenWrt `22.03+` with `nftables` is the practical baseline.

## Features

- Fast import flow for subscriptions, share links, and supported 3x-ui/Xray JSON files.
- Support for VLESS, VMess, and Trojan nodes.
- Safe runtime updates with `xray -test`, last-known-good backup, and controlled service reloads.
- Auto mode with health checks, live failover, anti-flap logic, and reboot-time runtime restore.
- Simple transparent routing rules for LAN hosts, CIDRs, ranges, or destination targets.
- Dedicated DNS commands with a sensible default profile for everyday router use.
- Shared state across CLI, LuCI, and TUI, so you can switch interfaces without losing context.

## Quick Start

Install the latest stable release on your router:

```bash
wget -O /tmp/routeflux-install.sh "https://github.com/Alaxay8/routeflux/releases/latest/download/install.sh" && sh /tmp/routeflux-install.sh
```

Then import a subscription and connect:

```bash
routeflux add https://provider.example/subscription
routeflux list subscriptions
routeflux connect --auto --subscription sub-1234567890
```

After installation you can use:

- LuCI: `Services -> RouteFlux`
- CLI: over SSH with `routeflux ...`
- TUI: `routeflux tui`

## Installation

### Install from a GitHub release

Use the latest stable installer:

```bash
wget -O /tmp/routeflux-install.sh "https://github.com/Alaxay8/routeflux/releases/latest/download/install.sh" && sh /tmp/routeflux-install.sh
```

To update an existing router install in place without losing subscriptions, custom service aliases, or presets stored in `/etc/routeflux`:

```bash
ROUTEFLUX_TAG=v0.1.5
wget -O /tmp/routeflux-install.sh "https://github.com/Alaxay8/routeflux/releases/download/${ROUTEFLUX_TAG}/install.sh" && sh /tmp/routeflux-install.sh
```

If you need a pinned release:

```bash
ROUTEFLUX_TAG=v0.1.5
wget -O /tmp/routeflux-install.sh "https://github.com/Alaxay8/routeflux/releases/download/${ROUTEFLUX_TAG}/install.sh" && sh /tmp/routeflux-install.sh
```

The installer automatically installs the bundled Xray runtime when the router does not already provide a usable Xray binary and service.
It updates RouteFlux in place and preserves existing `/etc/routeflux` state files.

Current easy-install release assets are published for:

- `mipsel_24kc`
- `x86_64`

To remove RouteFlux and the bundled Xray runtime:

```bash
wget -O /tmp/routeflux-uninstall.sh "https://github.com/Alaxay8/routeflux/releases/latest/download/uninstall.sh" && sh /tmp/routeflux-uninstall.sh
```

### Build from source

Requirements:

- Go `1.26` or later
- OpenWrt or ImmortalWrt with `nftables`

Build the local binary:

```bash
make build
```

Cross-build for OpenWrt:

```bash
make build-openwrt
make build-openwrt-x86_64
```

Create release artifacts:

```bash
make package-release
```

For a manual router install, copy the generated tarball to the router and extract it at `/`:

```bash
VERSION="$(git describe --tags --always --dirty | sed 's/^v//')"
scp -O "./dist/routeflux_${VERSION}_mipsel_24kc.tar.gz" root@router:/tmp/
ssh root@router "tar -xzf /tmp/routeflux_${VERSION}_mipsel_24kc.tar.gz -C / && rm -f /tmp/luci-indexcache && rm -rf /tmp/luci-modulecache && /etc/init.d/rpcd reload && /etc/init.d/uhttpd reload"
```

## Usage

### Everyday flow

```bash
routeflux add https://provider.example/subscription
routeflux list subscriptions
routeflux list nodes --subscription sub-1234567890
routeflux connect --subscription sub-1234567890 --node abcdef123456
routeflux status
routeflux disconnect
```

### Auto mode

```bash
routeflux connect --auto --subscription sub-1234567890
routeflux daemon
```

Use `routeflux daemon --once` when you want a single refresh and health scan without running a long-lived service.

On OpenWrt, enable the service when you want auto refresh, failover monitoring, and reboot-time restore:

```bash
/etc/init.d/routeflux enable
/etc/init.d/routeflux start
```

### DNS and firewall helpers

```bash
routeflux dns get
routeflux dns set default
routeflux dns explain

routeflux firewall get
routeflux firewall set hosts 192.168.1.150
routeflux firewall set targets youtube instagram 1.1.1.1
routeflux firewall set anti-target gosuslugi.ru sberbank.ru
routeflux services set openai openai.com chatgpt.com oaistatic.com
routeflux services list
routeflux firewall explain
```

### Other useful commands

```bash
routeflux refresh --all
routeflux diagnostics
routeflux logs
routeflux settings get
routeflux services list
routeflux version
routeflux tui
```

## Examples

Import a raw share link:

```bash
routeflux add 'vless://uuid@example.com:443?...#Example'
```

Import a valid 3x-ui or Xray JSON config:

```bash
routeflux add < ./client-config.json
```

Route one LAN device through the active RouteFlux connection:

```bash
routeflux firewall set hosts 192.168.1.150
routeflux connect --subscription sub-1234567890 --node 90c42d5dd302
```

Route your whole private LAN through RouteFlux:

```bash
routeflux firewall set hosts all
routeflux connect --subscription sub-1234567890 --node 90c42d5dd302
```

Use encrypted DNS for external domains while keeping local names on the router:

```bash
routeflux dns set default
```

Create a custom target alias once and reuse it in firewall targets:

```bash
routeflux services set openai openai.com chatgpt.com oaistatic.com
routeflux firewall set targets openai youtube
```

Keep selected banking or government sites direct and proxy the rest of the LAN traffic:

```bash
routeflux firewall set anti-target gosuslugi.ru sberbank.ru
routeflux connect --subscription sub-1234567890 --node 90c42d5dd302
```

## Configuration

By default, RouteFlux stores state under `/etc/routeflux` on OpenWrt. For local development it uses `./.routeflux`.

Useful environment variables:

- `ROUTEFLUX_ROOT`: override the state directory
- `ROUTEFLUX_XRAY_CONFIG`: override the generated Xray config path
- `ROUTEFLUX_XRAY_SERVICE`: override the Xray service control script
- `ROUTEFLUX_XRAY_BINARY`: override the Xray binary used for validation
- `ROUTEFLUX_FIREWALL_RULES`: override the generated nftables rules file path

Main persisted files:

- `/etc/routeflux/subscriptions.json`
- `/etc/routeflux/settings.json`
- `/etc/routeflux/state.json`

For guided explanations, prefer the built-in help:

- `routeflux dns explain`
- `routeflux firewall explain`
- `routeflux settings --help`

## DNS Modes

If you do not want to think about DNS details, use this:

```bash
routeflux dns set default
```

It is the best everyday option for most users: local names stay local, public DNS is encrypted.

- `system`: leave DNS as it is
  Example: your router DNS already works fine and you do not want RouteFlux to change it.

```bash
routeflux dns set mode system
```

- `remote`: send every DNS request to the DNS servers you choose
  Example: you want all DNS to go through Cloudflare or Google DNS.

```bash
routeflux dns set mode remote
routeflux dns set transport doh
routeflux dns set servers "1.1.1.1,1.0.0.1"
```

- `split`: keep local names on the router, send internet domains to your chosen DNS
  Example: `router.lan` stays local, `google.com` goes to encrypted DNS.

```bash
routeflux dns set default
```

- `disabled`: do not write RouteFlux DNS settings into Xray config
  Example: useful only for custom setups where DNS is managed somewhere else.

```bash
routeflux dns set mode disabled
```

DNS transports:

- `plain`: normal DNS, no encryption
- `doh`: encrypted DNS over HTTPS

## Firewall Modes

- `disabled`: do not redirect router traffic through RouteFlux
  Example: RouteFlux is installed, but no device is forced through the proxy.

```bash
routeflux firewall disable
```

What still works when firewall is disabled:

- you can add, refresh, remove, and inspect subscriptions
- you can connect to a node manually or in auto mode
- RouteFlux still generates and applies the Xray config for the selected node
- DNS settings still work
- CLI, LuCI, TUI, daemon, health checks, and failover still work

What does not happen when firewall is disabled:

- RouteFlux does not add nftables redirect rules
- router traffic is not sent through the proxy automatically
- LAN devices do not use the selected node until you enable `hosts`, `targets`, or `anti-target`
- transparent proxy mode is not enabled for intercepted traffic

In simple words: RouteFlux still manages subscriptions and the active Xray runtime, but it does not capture traffic from the router or your LAN by itself.

- `targets`: send traffic through RouteFlux only when the destination matches selected services, domains, or IPv4 targets
  Example: only traffic to specific services should go through the proxy.

```bash
routeflux firewall set targets youtube instagram 1.1.1.1
```

Target selectors:

- service preset: `discord`, `facetime`, `gemini`, `gemini-mobile`, `instagram`, `netflix`, `notebooklm`, `notebooklm-mobile`, `telegram`, `telegram-web`, `twitter`, `whatsapp`, `youtube`
- custom service alias: `openai`
- domain: `youtube.com`
- IPv4 address: `1.1.1.1`
- subnet: `8.8.8.0/24`
- range: `203.0.113.10-203.0.113.20`

Notes for domain targets:

- Create your own aliases with `routeflux services set <name> <domain-or-ip...>`, then use that alias in `routeflux firewall set targets ...`.
- Custom aliases can contain only domains, IPv4 addresses, CIDRs, and IPv4 ranges.
- Built-in preset names are reserved and stay read-only.
- RouteFlux treats `youtube.com` as the domain and its subdomains.
- Popular presets like `youtube`, `instagram`, `discord`, `twitter`, `netflix`, `whatsapp`, `gemini`, `gemini-mobile`, `notebooklm`, and `notebooklm-mobile` expand to the domain families they need.
- Popular root domains like `youtube.com`, `instagram.com`, `netflix.com`, `x.com`, `gemini.google.com`, and `notebooklm.google.com` still auto-expand to the domain families they need.
- Use `gemini-mobile` and `notebooklm-mobile` for the Android or iOS apps when the web presets are too narrow.
- The mobile Google AI presets may include a small set of IPv4 targets in addition to domains because some app traffic does not expose a usable hostname.
- `gemini`, `gemini-mobile`, `notebooklm`, `notebooklm-mobile`, `telegram`, `facetime`, `twitter`, and `netflix` are best-effort presets because those apps may use direct IPs or broader shared vendor infrastructure.
- The mobile Google AI presets are intentionally broader and may also catch shared Google infrastructure. If they are still not enough for your device, add the missing Google domains as a custom alias and route that alias instead.
- Domain targets require `dnsmasq` with `nftset` support, which usually means `dnsmasq-full` on OpenWrt.
- Domain targets depend on router-visible DNS answers. If clients use their own DoH or DoT directly, target IP sets may stay empty.
- On shared CDNs, RouteFlux now falls back to direct routing for non-matching transparent traffic instead of sending every matched IP through the selected node.

- `anti-target`: keep selected services, domains, or IPv4 targets direct and send all other LAN traffic through RouteFlux
  Example: keep banking or government sites direct while the rest of the LAN uses the proxy.

```bash
routeflux firewall set anti-target gosuslugi.ru sberbank.ru
```

Anti-target selectors:

- service preset: `discord`, `facetime`, `gemini`, `gemini-mobile`, `instagram`, `netflix`, `notebooklm`, `notebooklm-mobile`, `telegram`, `telegram-web`, `twitter`, `whatsapp`, `youtube`
- custom service alias: `openai`
- domain: `gosuslugi.ru`
- IPv4 address: `1.1.1.1`
- subnet: `8.8.8.0/24`
- range: `203.0.113.10-203.0.113.20`

Notes for anti-target:

- Anti-target uses the same selector parsing and alias expansion as `targets`.
- Anti-target is still best-effort and currently considered unstable. Some browsers, mobile apps, CDNs, and QUIC-heavy services may still require manual testing before you rely on it full time.
- Domain anti-targets do not require `dnsmasq` nftset support because Xray matches them from transparent traffic sniffing.
- Anti-target is designed for LAN clients. It does not redirect router-originated traffic.
- `block-quic` is now a legacy compatibility flag for older TCP-only setups. Current LAN transparent routing intercepts QUIC directly.

- `hosts`: send all traffic from selected LAN devices through RouteFlux
  Example: route one phone, TV, or laptop through the proxy.

```bash
routeflux firewall set hosts 192.168.1.150
```

Host selectors:

- one device: `192.168.1.150`
- subnet: `192.168.1.0/24`
- range: `192.168.1.150-192.168.1.159`
- whole private LAN: `all`

Examples:

```bash
routeflux firewall set hosts 192.168.1.0/24
routeflux firewall set hosts 192.168.1.150-192.168.1.159
routeflux firewall set hosts all
```

Other firewall options:

- `block-quic`: legacy compatibility flag for older TCP-only routing setups
- `port`: changes the transparent redirect port

## Development

Format, vet, and test:

```bash
make fmt
make lint
go test ./...
```

Build and runtime coverage:

```bash
make build
make coverage-runtime
```

OpenWrt integration suite:

```bash
make test-integration
```

Additional project docs:

- [docs/config.md](docs/config.md)
- [docs/architecture.md](docs/architecture.md)
- [docs/tui-flow.md](docs/tui-flow.md)

## License

MIT
