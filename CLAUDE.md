# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Run all tests
go test ./...

# Run a single package's tests
go test github.com/dobyte/due/v2/session
go test github.com/dobyte/due/v2/packet

# Run a specific test
go test -run TestXxx github.com/dobyte/due/v2/packet

# Tidy dependencies
./tidy.sh
```

## Architecture Overview

**due** is a distributed game server framework. The top-level entry point is `Container` (`container.go`), which manages the lifecycle of all registered `Component` instances (Init → Start → Close → Destroy).

### Cluster Node Types

Three server roles, all implementing `component.Component`:

| Role | Package | Responsibility |
|------|---------|----------------|
| **Gate** | `cluster/gate` | Manages client connections; receives packets and routes to Node |
| **Node** | `cluster/node` | Core game logic; stateful or stateless; handles routed messages |
| **Mesh** | `cluster/mesh` | Stateless microservice; callable from Node via RPC |

Gate holds a `session.Session` (connection manager). Node holds a `Router` (message dispatcher), `Trigger` (event dispatcher), `Scheduler` (timer), and optionally `Actor` instances.

### Message Flow

```
Client → [TCP/KCP/WS] → Gate (network.Conn → session.Session)
       → [internal transporter / gRPC/rpcx] → Node (Router → Handler)
       → [transport.Client] → Mesh (RPC)
```

The wire protocol is `size(4B) + header(1B) + route(2B) + seq(2B) + data`, defined in `packet/`.

### Key Abstractions

- **`transport`** (`transport/transporter.go`): Interface for inter-node RPC. Implementations: `transport/grpc`, `transport/rpcx`. Used by Node/Mesh for service calls.
- **`network`** (`transport/` tcp/kcp/ws subdirs): Client-facing network layer. Implementations provide `network.Conn` and `network.Server`.
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

`internal/transporter/` contains the private Gate↔Node communication protocol (distinct from the public `transport/` RPC layer used for Mesh). `internal/link/` handles the routing table that maps a UID to its current Gate and Node.

### Module Path

`github.com/dobyte/due/v2` — this is a v2 module. All imports must use this prefix.
