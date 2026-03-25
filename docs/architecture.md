# RouteFlux Architecture

## Overview
RouteFlux separates subscription parsing, persistence, health evaluation, runtime integration, and interface concerns. The architecture keeps business logic independent from the CLI and TUI so the project can later add RPC or LuCI layers without rewriting core behavior.

## Layers
### Domain
Pure models for subscriptions, nodes, settings, runtime state, health, and score results.

### Parser
Protocol detection, base64 decoding, mixed-payload parsing, and normalization for provider nodes.

### Store
Atomic JSON persistence for subscriptions, settings, and runtime state.

### Probe
TCP connect probing, rolling health updates, score calculation, and anti-flap switching.

### Backend
Xray config generation, file writing, and runtime service control abstractions.

### Application
Workflows for add, refresh, connect, disconnect, auto selection, and background refresh scheduling.

### Interfaces
Scriptable Cobra CLI, a background daemon mode for subscription refresh, and keyboard-driven Bubble Tea TUI.

## Future Extensions
- sing-box backend implementation behind the same backend interface
- richer router dataplane integration and policy routing helpers
