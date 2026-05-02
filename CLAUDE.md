# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## On Session Start

Read `LEARNING.md` at the beginning of every session. It tracks the current learning progress across five steps. Use it to understand where the user left off and tailor responses accordingly — e.g. if Step 2 is in progress, focus on tracing message flow rather than giving broad overviews.

> Note: This repository is currently used for **source code study only**, not active development. Prioritize explanation and tracing over code modification.

## Commands

```bash
# Run all tests
go test ./...

# Run a single package's tests
go test github.com/dobyte/due/v2/session
go test github.com/dobyte/due/v2/packet

# Run a specific test
go test -run TestXxx github.com/dobyte/due/v2/packet

# Tidy dependencies (runs go mod tidy across all sub-modules)
./tidy.sh
```

Each sub-module (e.g. `network/tcp`, `transport/grpc`) has its own `go.mod`. Always run `./tidy.sh` after adding or removing dependencies.

## Module Path

`github.com/dobyte/due/v2` — this is a v2 module. All imports must use this prefix.

Sub-module imports use paths like `github.com/dobyte/due/network/ws/v2`. If sub-module and main module versions diverge, pin the sub-module to the matching commit hash: `go get github.com/dobyte/due/lock/redis/v2@<commit>`.

## Architecture Overview

**due** is a distributed game server framework. The top-level entry point is `Container` (`container.go`), which manages the lifecycle of all registered `Component` instances (Init → Start → Close → Destroy).

### Cluster Node Types

Four roles, all implementing `component.Component`:

| Role | Package | Responsibility |
|------|---------|----------------|
| **Gate** | `cluster/gate` | Manages client connections; receives packets and routes to Node |
| **Node** | `cluster/node` | Core game logic; stateful or stateless; handles routed messages |
| **Mesh** | `cluster/mesh` | Stateless microservice; callable from Node via RPC |
| **Client** | `cluster/client` | Built-in test/debug client; connects to Gate directly |

Each role exposes a `Proxy` (e.g. `node.Proxy`, `gate.Proxy`) — the surface area for business logic. Never access node internals directly; always go through `Proxy`.

Gate holds a `session.Session` (connection manager). Node holds a `Router` (message dispatcher), `Trigger` (event dispatcher), `Scheduler` (timer), and optionally `Actor` instances.

### Message Flow

```
Client → [TCP/KCP/WS] → Gate (network.Conn → session.Session)
       → [internal/transporter] → Node (Router → Handler)
       → [transport.Client] → Mesh (RPC)
```

The wire protocol is `size(4B) + header(1B) + route(2B) + seq(2B) + data`, defined in `packet/`. Route and seq byte widths are configurable per packer instance.

### Actor Model (Node only)

`cluster/node/actor.go` — each `Actor` owns a mailbox channel and a `Scheduler`. Messages are processed sequentially inside the actor, eliminating per-message locking. Implement the `Processor` interface and register via `Creator`. Actors are spawned/destroyed through `node.Proxy`.

### Key Abstractions

- **`transport`** (`transport/transporter.go`): Interface for inter-node RPC. Implementations: `transport/grpc`, `transport/rpcx`. Used by Node/Mesh for service calls.
- **`network`** (`network/` tcp/kcp/ws subdirs): Client-facing network layer. Implementations provide `network.Conn` and `network.Server`.
- **`registry`** (`registry/`): Service discovery interface. Implementations: consul, etcd, nacos.
- **`locate`** (`locate/`): Tracks which Gate and Node a user (UID) is connected to. Implementation: redis.
- **`eventbus`** (`eventbus/`): Async event pub/sub between nodes. Implementations: redis, nats, kafka, rabbitmq.
- **`session`** (`session/session.go`): In-memory mapping of CID→Conn and UID→Conn within a Gate, plus channel pub/sub.
- **`config`** (`config/`): Dynamic config with hot-reload. Sources: consul, etcd, nacos, file (json/yaml/toml/xml).
- **`etc`** (`etc/`): Static startup config (env vars / config files read once at boot).
- **`lock`** (`lock/`): Distributed lock interface. Implementations: redis, memcache.
- **`cache`** (`cache/`): Cache interface. Implementations: redis, memcache.
- **`task`** (`task/`): Goroutine pool for async task execution (wraps `ants`).

### Internal Layer

`internal/transporter/` contains the private Gate↔Node communication protocol (distinct from the public `transport/` RPC layer used for Mesh). `internal/link/` handles the routing table that maps a UID to its current Gate and Node — split into `GateLinker` and `NodeLinker`.

## Upstream Sync

This repo is a private fork of `dobyte/due`. Local commits use the `[local]` prefix. To pull upstream changes:

```bash
git fetch upstream
git rebase upstream/main local-patches
# On conflict: resolve → git add → git rebase --continue
# After rebase: git push origin local-patches --force-with-lease
```

See `UPSTREAM.md` for full workflow.
