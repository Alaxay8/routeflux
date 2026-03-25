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

## Settings
- `refresh_interval`: subscription refresh cadence
- `health_check_interval`: active probe cadence
- `switch_cooldown`: minimum delay between auto switches
- `latency_threshold`: minimum improvement required to switch healthy nodes
- `auto_mode`: whether auto selection is enabled
- `mode`: current selection mode
- `log_level`: backend and app log verbosity (`debug`, `info`, `warn`, or `error` at startup)

## State
Runtime state keeps:
- active subscription and node
- current mode
- connection flag
- last refresh timestamps
- node health telemetry
- last switch time
- last success and failure data

## Upgrade And Recovery
- RouteFlux preserves `/etc/routeflux` during in-place upgrades.
- Missing or older state/settings schema versions are upgraded to the current schema during load.
- Malformed `settings.json` or `state.json` is renamed to `*.corrupt-<UTC>.json`, replaced with a fresh canonical file, and reported through the logger as a recovery warning.
- Future schema versions are not downgraded automatically and remain a hard error.
