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
- `log_level`: backend and app log verbosity

## State
Runtime state keeps:
- active subscription and node
- current mode
- connection flag
- last refresh timestamps
- node health telemetry
- last switch time
- last success and failure data
