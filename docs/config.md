# RouteFlux Configuration

## Paths
Default OpenWrt paths:
- `/etc/routeflux/subscriptions.json`
- `/etc/routeflux/settings.json`
- `/etc/routeflux/state.json`
- `/etc/xray/config.json`

Local development paths:
- `./.routeflux/subscriptions.json`
- `./.routeflux/settings.json`
- `./.routeflux/state.json`
- `./.routeflux/xray-config.json`

RouteFlux hardens secret-bearing state on disk:
- RouteFlux-owned state directories use `0700`
- RouteFlux state files, lock files, Xray live config, and Xray last-known-good backups use `0600`
- Existing installs are hardened during startup and in-place upgrades

## Settings
- `refresh_interval`: subscription refresh cadence
- `health_check_interval`: active probe cadence for daemon-backed auto health monitoring
- `switch_cooldown`: minimum delay between auto switches while the daemon is monitoring auto mode
- `latency_threshold`: minimum improvement required to switch healthy nodes while the daemon is monitoring auto mode
- `auto_mode`: whether auto selection is enabled
- `mode`: current selection mode
- `log_level`: backend and app log verbosity (`debug`, `info`, `warn`, or `error` at startup)

Auto health checks and live failover are performed only while `routeflux daemon` or the OpenWrt `/etc/init.d/routeflux` service is running.

## Local and LAN proxy

The `proxy` block in `settings.json` stores explicit proxy listener settings:

```json
{
  "proxy": {
    "allow_lan": false,
    "socks_port": 10808,
    "http_port": 10809
  }
}
```

Use `routeflux proxy set --allow-lan true --socks-port 10808 --http-port 10809`
or LuCI **Settings → Local / LAN Proxy** to validate and apply changes together.
`routeflux --json proxy get` returns this block. Omitted command flags keep their
saved values. Older settings migrate to localhost-only listeners on the default
ports. Schema version 11 adds the proxy block.

LAN access switches both listeners from `127.0.0.1` to `0.0.0.0` (IPv4). It is
independent of Routing and does not change OpenWrt firewall rules. Restrict access
to trusted LAN clients; these listeners do not require authentication. The selected
upstream must support UDP for SOCKS5 UDP relay to work.

## State
Runtime state keeps:
- active subscription and node
- current mode
- connection flag
- last refresh timestamps stored in UTC
- node health telemetry
- last switch time
- last success and failure data

## Upgrade And Recovery
- RouteFlux preserves `/etc/routeflux` during in-place upgrades.
- Missing or older state/settings schema versions are upgraded to the current schema during load.
- Malformed `settings.json` or `state.json` is renamed to `*.corrupt-<UTC>.json`, replaced with a fresh canonical file, and reported through the logger as a recovery warning.
- Future schema versions are not downgraded automatically and remain a hard error.
