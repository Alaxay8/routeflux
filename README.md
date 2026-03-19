# RouteFlux

## Overview
RouteFlux is a lightweight OpenWrt-native Go application for managing proxy subscriptions and generating Xray runtime configuration. It aims to give routers and edge devices a simple subscription-driven experience similar to modern desktop VPN or proxy clients without forcing users to hand-edit Xray JSON.

## Features
- Import raw `vless://`, `vmess://`, `trojan://`, and parser-ready `ss://` nodes.
- Import mixed subscription payloads, including base64-encoded provider responses.
- Persist subscriptions, settings, active state, and health telemetry with atomic JSON writes.
- Select nodes manually or use automatic best-node selection with health and anti-flap logic.
- Generate Xray configs and reload the runtime through an OpenWrt init.d service abstraction.
- Use either the scriptable Cobra CLI or the interactive Bubble Tea terminal UI.

## Quick Start
```bash
make build
./bin/routeflux add --raw 'vless://uuid@example.com:443?...#Example'
./bin/routeflux list subscriptions
./bin/routeflux tui
```

## Installation
1. Install Go 1.22 or later.
2. Install Xray Core on the target router.
3. Build RouteFlux:
```bash
make build
```
4. Copy the binary to the router:
```bash
scp ./bin/routeflux root@router:/usr/bin/routeflux
```
5. Ensure the Xray init script exists at `/etc/init.d/xray` or override `ROUTEFLUX_XRAY_SERVICE`.

## Usage
CLI examples:
```bash
routeflux add --url https://provider.example/subscription
routeflux add --raw "$(cat subscription.txt)"
routeflux list subscriptions
routeflux list nodes --subscription sub-1234567890
routeflux refresh --subscription sub-1234567890
routeflux refresh --all
routeflux connect --subscription sub-1234567890 --node abcdef123456
routeflux connect --auto --subscription sub-1234567890
routeflux disconnect
routeflux status
routeflux settings get
routeflux settings set refresh-interval 1h
routeflux settings set auto-mode true
routeflux tui
```

## Examples
Manual connect:
```bash
routeflux connect --subscription sub-8b9f930214 --node 90c42d5dd302
```

Auto mode:
```bash
routeflux connect --auto --subscription sub-8b9f930214
```

## Configuration
RouteFlux stores state under `/etc/routeflux` on OpenWrt by default. For local development it uses `./.routeflux`.

Important environment variables:
- `ROUTEFLUX_ROOT`: override the state directory.
- `ROUTEFLUX_XRAY_CONFIG`: override the generated Xray config path.
- `ROUTEFLUX_XRAY_SERVICE`: override the Xray service control script.

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
- The parser and selector are heavily unit tested first and use realistic fixtures.
- Xray config generation uses golden files.
- Atomic writes use temp files plus rename to avoid corrupting state on power loss.

## Architecture
The codebase is split into domain, parser, store, probe, backend, app, CLI, and TUI layers. See [docs/architecture.md](/Users/alexey/dev/route-flux/docs/architecture.md) for the full breakdown.

## TUI
The MVP TUI is keyboard-driven and focuses on fast subscription selection:
- `j` / `k`: move between subscriptions
- `h` / `l`: move between nodes
- `c`: connect selected node
- `a`: enable auto selection on the selected subscription
- `r`: refresh selected subscription
- `s`: open settings
- `d`: disconnect
- `q`: quit

Placeholder screenshots:
- `docs/images/tui-main.txt`
- `docs/images/tui-settings.txt`

## Supported Protocols
- VLESS
- VMess
- Trojan
- Shadowsocks parsing scaffold

## OpenWrt Deployment
1. Build with `make build-openwrt`.
2. Copy the binary to the router.
3. Create `/etc/routeflux` if it does not exist.
4. Ensure Xray is installed and configured to accept generated config reloads.
5. Run `routeflux tui` or use the CLI to import a subscription and connect.

## Limitations
- Transparent router traffic interception is not fully automated in MVP.
- Health probing currently uses TCP connect checks, not full HTTP-through-proxy validation.
- Auto mode keeps RouteFlux as the source of truth and does not yet drive advanced Xray observatory features.

## Roadmap
- Add sing-box backend support.
- Add richer health probes and passive traffic health signals.
- Integrate OpenWrt package metadata and procd service files.
- Expand subscription management commands for rename and removal.
- Add export and diagnostics commands for support workflows.

## License
MIT
