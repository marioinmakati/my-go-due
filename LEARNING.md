# 代码库学习计划

## 进度总览

| 步骤 | 内容 | 重要性 | 状态 | 完成日期 |
|------|------|--------|------|----------|
| 第一步 | 生成代码库概览 | ⭐ 基础 | ✅ 已完成 | 2026-05-02 |
| 第二步 | 梳理核心流程 | 🔥 最重要 | ✅ 已完成 | 2026-05-02 |
| 第三步 | 逐模块深入 | 🔥 重要 | ✅ 已完成 | 2026-05-02 |
| 第四步 | 动手练习 | ⏸ 二开时再做 | ⬜ 未开始 | — |
| 第五步 | 结合二次开发目标 | ⏸ 二开时再做 | ⬜ 未开始 | — |

> 状态说明：⬜ 未开始 / 🔄 进行中 / ✅ 已完成
> 重要性说明：🔥 纯读源码必做 / ⭐ 基础铺垫 / ⏸ 有二开需求时再开启

**附录章节（随时可查）**
- [本地运行指南](#本地运行指南) — 启动中间件、Gate、Node 的完整步骤
- [二次开发指南](#二次开发指南) — 扩展点总览 + 6 个业务场景代码示例
- [新人源码阅读路线](#新人源码阅读路线) — 4 个阶段，从框架概念到跑通链路
- [调试工具指南](#调试工具指南) — Bruno 适用范围、cluster/client 调试游戏协议、grpcurl 调试 Mesh

---

## 第一步：生成代码库概览 ✅

运行以下命令，让 Claude 扫描项目并生成 `CLAUDE.md`：

```
/init
```

**目标：** 得到项目结构、核心模块、入口文件的整体地图。

**完成标志：** `CLAUDE.md` 生成完毕，能说出各顶层目录的职责。

**完成记录（2026-05-02）**
- `CLAUDE.md` 已生成并补充：Proxy 层说明、Actor 模型、cluster/client 角色、子模块版本对齐规则、上游同步工作流。
- 核心顶层目录职责已梳理完毕，见 `CLAUDE.md` 的 Key Abstractions 节。

---

## 第二步：梳理核心流程 🔄

**目标：** 理解主干数据流和模块边界，能独立描述一条消息的完整生命周期。

### 子任务进度

| # | 子任务 | 状态 | 完成日期 |
|---|--------|------|----------|
| 2-1 | Container 生命周期：Init → Start → Close → Destroy | ✅ | 2026-05-02 |
| 2-2 | 客户端连接建立：network.Server → network.Conn → session.Session | 🔄 | — |
| 2-3 | 上行消息路径：Gate 收包 → internal/transporter → Node Router → Handler | 🔄 | — |
| 2-4 | 下行消息路径：Node 推送 → GateLinker → session → network.Conn | 🔄 | — |
| 2-5 | 服务注册与发现：registry 注册实例、locate 定位 UID 所在 Gate/Node | 🔄 | — |
| 2-6 | Node → Mesh RPC 调用链：transport.Client → Mesh Handler | 🔄 | — |

### 子任务详情

#### 2-1 Container 生命周期

**关键文件：** `container.go`

**调用链：**
```
Container.Serve()
  ├── doInitComponents()    → comp.Init()         [顺序执行]
  ├── doStartComponents()   → comp.Start()        [顺序执行]
  ├── doWaitSystemSignal()  → 阻塞等待 SIGINT/SIGTERM
  ├── doCloseComponents()   → comp.Close()        [并发, 受 shutdownMaxWaitTime 限制]
  ├── doDestroyComponents() → comp.Destroy()      [并发, 5s 超时]
  └── doClearModules()      → 关闭 eventbus/lock/cache/task/config/etc/log
```

**提问模板：**
- "Close 和 Destroy 为什么用并发而不是顺序？"
- "shutdownMaxWaitTime 从哪里读取？"

**完成标志：** 能说出 Init/Start 是顺序的、Close/Destroy 是并发的，以及原因。

---

#### 2-2 客户端连接建立

**关键文件：**
- `network/tcp/server.go` — Start() 启动监听
- `cluster/gate/gate.go` — handleConnect() 行 145-154
- `session/session.go` — AddConn() 行 47-59

**调用链：**
```
network.Server.Start()
  └── accept loop → 生成 network.Conn (serverConn)
        └── gate.handleConnect(conn)
              ├── session.AddConn(conn)  → conns[cid] = conn
              └── (uid != 0) users[uid] = conn
```

**提问模板：**
- "WS/KCP 的连接建立和 TCP 有什么不同？"
- "CID 和 UID 的区别是什么，什么时候 UID 才有值？"

**完成标志：** 能解释 CID（连接级）与 UID（用户级）的区别，以及何时绑定。

---

#### 2-3 上行消息路径

**关键文件：**
- `cluster/gate/gate.go` — handleReceive() 行 173-178
- `cluster/gate/proxy.go` — deliver() 行 78-103
- `internal/link/node.go` — Deliver() 行 174-221
- `internal/transporter/node/server.go` — deliver() 行 62-88
- `cluster/node/router.go` — deliver() 行 141-153，handle() 行 163-192

**调用链：**
```
gate.handleReceive(cid, uid, data)
  └── proxy.deliver(ctx, cid, uid, data)
        ├── packet.UnpackMessage(data)  → 解析 route/seq/body
        └── nodeLinker.Deliver(ctx, DeliverArgs)
              └── internal/link/node: 按 route 定位目标 Node
                    └── transporter/node/client.Deliver()  [内部 RPC]
                          └── transporter/node/server.deliver()
                                └── node.provider.Deliver()
                                      └── router.deliver() → reqChan
                                            └── router.handle()
                                                  └── RouteHandler(ctx)
```

**提问模板：**
- "如果消息的 NID 为空，NodeLinker 怎么决定发给哪个 Node？"
- "reqChan 的作用是什么，为什么不直接调用 handler？"

**完成标志：** 能独立描述从 `handleReceive` 到 `RouteHandler` 的完整路径，说出每一跳的文件名。

---

#### 2-4 下行消息路径

**关键文件：**
- `cluster/node/proxy.go` — Push() 系列方法
- `internal/link/gate.go` — Push() 行 328-339，Multicast() 行 366-414
- `internal/transporter/gate/client.go` — Push()
- `internal/transporter/gate/server.go` — push()
- `session/session.go` — Push() 行 219-238
- `network/tcp/server_conn.go` — Push() → taskQueue → 写 TCP

**调用链：**
```
node.Proxy.Push(ctx, uid, message)
  └── gateLinker.Push(ctx, PushArgs)
        └── link/gate: 定位 UID 所在 GateID
              └── transporter/gate/client.Push()  [内部 RPC]
                    └── transporter/gate/server.push()
                          └── gate.provider.Push()
                                └── session.Push(kind, target, msg)
                                      └── conn.Push(msg)
                                            └── taskQueue → net.Conn.Write()
```

**提问模板：**
- "doDirectMulticast 和 doIndirectMulticast 分别在什么场景下用？"
- "session.Push 里的 kind 参数区分了什么？"

**完成标志：** 能解释为什么 Node 不直接写连接，而要绕一圈经过 Gate。

---

#### 2-5 服务注册与 UID 定位

**关键文件：**
- `cluster/gate/gate.go` — registerServiceInstance() 行 206-224
- `locate/locator.go` — Locator 接口定义
- `cluster/gate/proxy.go` — bindGate()
- `internal/link/node.go` — BindNode() 行 114-129，LocateNode() 行 81-103

**调用链：**
```
启动时：
  gate.registerServiceInstance()
    └── registry.Register(ctx, ServiceInstance{ID, Name="gate", Endpoint})

连接时：
  proxy.bindGate(ctx, uid, gateID)
    └── locator.BindGate(ctx, uid, gid)  → Redis 写入 uid→gateID

路由时：
  link/node.LocateNode(ctx, uid, name)
    ├── 本地缓存命中 → 直接返回 nid
    └── 缓存未命中  → locator.LocateNode(ctx, uid, name)
```

**提问模板：**
- "locate 和 registry 有什么区别，各自解决什么问题？"
- "Node 的本地 UID 缓存什么时候会失效？"

**完成标志：** 能区分 registry（服务实例发现）和 locate（用户位置追踪）的职责边界。

---

#### 2-6 Node → Mesh RPC 调用链

**关键文件：**
- `internal/link/node.go` — doRPC() 行 268-325
- `internal/transporter/node/client.go` — Deliver() / Trigger()
- `cluster/mesh/mesh.go` — Start() 行 80-95
- `transport/transporter.go` — Server/Client 接口定义

**调用链：**
```
RouteHandler 中调用 ctx.CallNode(route, req, res)
  └── proxy.nodeLinker.doRPC(ctx, route, uid, fn)
        ├── dispatcher.FindRoute(routeID)      → 找到目标服务名
        ├── locator.LocateNode(uid, name)      → 找到目标 nid
        ├── registry.GetServiceInstance(nid)   → 拿到 Endpoint
        └── fn(ctx, transport.Client)
              └── transport.Client.Call()  [gRPC 或 rpcx]
                    └── Mesh.transport.Server 接收
                          └── 注册的 service handler 处理
```

**提问模板：**
- "Node 调用另一个 Node 和调用 Mesh 的代码路径有什么不同？"
- "transport.Client 怎么知道用 gRPC 还是 rpcx？"

**完成标志：** 能说出 Mesh 与 Node 在 RPC 层的共同点（都走 transport）和区别（有无状态、有无 locate 定位）。

---

### 整体完成标志

能填上具体函数名，独立描述下图完整路径：

```
Client
  ↓ TCP/KCP/WS
network.Server → network.Conn
  ↓ gate.handleReceive
session.Session (CID→Conn)
  ↓ proxy.deliver → nodeLinker
internal/transporter (Gate→Node 内部协议)
  ↓ router.handle
node.Router → RouteHandler
  ↓ (可选) doRPC → transport.Client
Mesh Handler
```

### 完成记录

**2-1（2026-05-02）**
- Init/Start 顺序执行（按 Add 顺序，保证启动依赖）
- Close/Destroy 并发执行（各组件互相独立，加快退出）
- Close 超时由 `etc.shutdownMaxWaitTime` 控制，Destroy 固定 5s
- 超时后不强制停止 goroutine，只是不再等待
- 注释已添加：`container.go` Serve()、`utils/xcall/goroutines.go` Run()

**2-2 至 2-6（2026-05-02）注释已就位，待口头验证**
- 已为所有关键文件添加调用链注释（见各文件函数头部注释）
- 关键文件清单：
  - 2-2: `network/tcp/server.go`, `cluster/gate/gate.go`, `session/session.go`
  - 2-3: `cluster/gate/proxy.go`, `internal/link/node.go`, `internal/transporter/node/client.go`, `internal/transporter/node/server.go`, `cluster/node/router.go`
  - 2-4: `internal/link/gate.go`, `internal/transporter/gate/client.go`, `internal/transporter/gate/server.go`, `session/session.go`
  - 2-5: `cluster/gate/gate.go`, `cluster/gate/proxy.go`, `locate/locator.go`, `registry/registry.go`, `internal/link/node.go`
  - 2-6: `cluster/mesh/mesh.go`, `transport/transporter.go`
- 待验证：按各子任务"完成标志"，用自己的话复述调用链后标为 ✅

---

## 第三步：逐模块深入 ⬜

**目标：** 对每个模块，理解它的接口设计、内部结构、与其他模块的依赖关系。

### 模块进度

| 模块 | 状态 | 完成日期 | 一句话总结 |
|------|------|----------|-----------|
| `packet` — 协议编解码 | ✅ | 2026-05-02 | size+header+route+seq+data 可配置字节数，最高位区分心跳/数据 |
| `session` — 连接管理 | ✅ | 2026-05-02 | CID/UID 双索引 + 频道订阅，Bind 时处理顶号登录 |
| `transport` — 节点间 RPC | ✅ | 2026-05-02 | Node↔Mesh 公开 RPC 层，支持 gRPC/rpcx 切换，与 internal/transporter 完全独立 |
| `network` — 客户端网络层 | ✅ | 2026-05-02 | 只负责字节流收发，不感知业务协议，Gate 层才做 packet 解包 |
| `registry` — 服务注册与发现 | ✅ | 2026-05-02 | 追踪服务实例及其 Endpoint，Node 注册时携带 Routes 供 NodeLinker 构建路由表 |
| `locate` — 用户位置追踪 | ✅ | 2026-05-02 | 追踪 UID→GateID/NodeID，本地缓存+Redis 两级查找 |
| `eventbus` — 异步事件总线 | ✅ | 2026-05-02 | fire-and-forget 广播，适合玩家上下线通知等无需返回值的跨节点事件 |
| `config` — 动态配置热更新 | ✅ | 2026-05-02 | Source.Watch 长连接推送变更，双 buffer 原子交换，Watch 回调通知业务层 |
| `cache` / `lock` — 基础设施 | ✅ | 2026-05-02 | 全局单例工厂模式，lock 用于 locate 写入时防竞态，cache 提供原子计数和防击穿 GetSet |

### 每个模块的学习路径

每个模块按以下顺序学习：

1. **读接口** — 找到模块根目录的接口定义文件（通常是 `xxx.go` 或 `interface.go`），理解对外暴露了哪些方法
2. **读实现** — 选一个具体实现（如 redis）读懂核心逻辑
3. **找调用方** — 搜索谁在调用这个接口，理解它在整体中的位置
4. **口头总结** — 用自己的话复述，让 Claude 纠正

提问模板：
```
session 模块对外暴露了哪些接口？关键文件在哪里？
谁会调用 eventbus？它和直接调用 locate 有什么区别？
帮我总结 transport 模块的设计，我来说说我的理解，你纠正
```

### 模块详情

#### 3-1 packet — 协议编解码

**关键文件：** `packet/` 目录

**学习重点：**
- 数据包格式：`size(4B) + header(1B) + route(2-4B) + seq(0-4B) + data`
- `Packer` 接口：Pack / Unpack
- routeBytes / seqBytes 的可配置性

**提问模板：**
- "心跳包和数据包在格式上有什么不同？"
- "route 字节数怎么配置，默认是多少？"

**完成标志：** 能手绘数据包的二进制布局，说出各字段的字节数和含义。

---

#### 3-2 session — 连接管理

**关键文件：** `session/session.go`

**学习重点：**
- 双索引结构：`conns map[CID]Conn` + `users map[UID]Conn`
- Push / Multicast / Broadcast 的区别
- channel 订阅机制

**提问模板：**
- "Broadcast 和 Multicast 在实现上有什么不同？"
- "一个 UID 同时在线多个设备，session 怎么处理？"

**完成标志：** 能解释 CID/UID 双索引的设计动机，以及 Push/Multicast/Broadcast 的适用场景。

---

#### 3-3 transport — 节点间 RPC

**关键文件：**
- `transport/transporter.go` — Server/Client 接口
- `transport/grpc/` — gRPC 实现
- `transport/rpcx/` — rpcx 实现

**学习重点：**
- `transport.Server` 接口：RegisterService / Start / Stop
- `transport.Client` 接口：Call / Send
- gRPC 和 rpcx 两种实现的切换方式

**提问模板：**
- "transport.Client 的 Call 和 Send 有什么区别？"
- "怎么从 gRPC 切换到 rpcx，需要改哪些地方？"

**完成标志：** 能说出 transport 层与 internal/transporter 层的职责边界（一个是 Node/Mesh 间的公开 RPC，一个是 Gate/Node 间的内部协议）。

---

#### 3-4 network — 客户端网络层

**关键文件：**
- `network/` 目录下的 `server.go` / `conn.go` 接口定义
- `network/tcp/` — TCP 实现
- `network/ws/` — WebSocket 实现
- `network/kcp/` — KCP 实现

**学习重点：**
- `network.Server` 接口：Start / Stop / OnConnect / OnReceive / OnDisconnect
- `network.Conn` 接口：ID / UID / Bind / Push / Close
- 三种协议实现的差异

**提问模板：**
- "network.Conn 的 Push 和 Send 有区别吗？"
- "KCP 和 TCP 在连接管理上有什么差异？"

**完成标志：** 能说出 network 层只负责字节流收发，不理解业务协议，业务解包在 Gate 层做。

---

#### 3-5 registry — 服务注册与发现

**关键文件：**
- `registry/registry.go` — Registry / Discovery 接口
- `registry/consul/` 或 `registry/etcd/` — 任选一个实现阅读

**学习重点：**
- `ServiceInstance` 结构：ID / Name / Kind / Endpoint / Routes / Events
- Register / Deregister / Watch / Services 方法
- Gate 和 Node 注册时携带的不同字段

**提问模板：**
- "ServiceInstance 里的 Routes 字段有什么用？"
- "Watch 机制是轮询还是长连接？"

**完成标志：** 能解释为什么 Node 注册时要携带 Routes 信息，以及 NodeLinker 如何利用这些信息做路由。

---

#### 3-6 locate — 用户位置追踪

**关键文件：**
- `locate/locator.go` — Locator 接口
- `locate/redis/` — Redis 实现

**学习重点：**
- BindGate / LocateGate / BindNode / LocateNode 四个核心方法
- 与 registry 的区别：registry 追踪服务实例，locate 追踪用户
- 本地缓存 + Redis 的两级查找

**提问模板：**
- "用户断线重连时，locate 里的记录怎么处理？"
- "LocateNode 里的 name 参数是什么？"

**完成标志：** 能画出 UID → GateID / NID 的查找路径，包含缓存和 Redis 两层。

---

#### 3-7 eventbus — 异步事件总线

**关键文件：**
- `eventbus/eventbus.go` — Eventbus 接口
- `eventbus/redis/` 或 `eventbus/nats/` — 任选一个实现

**学习重点：**
- Publish / Subscribe / Unsubscribe 方法
- 与直接 RPC 调用的区别：异步、解耦、跨节点广播
- 典型使用场景：玩家上线/下线通知其他节点

**提问模板：**
- "Gate 和 Node 通过 eventbus 通信的场景有哪些？"
- "eventbus 的消息和 transport 的 RPC 调用在什么时候选哪个？"

**完成标志：** 能举一个具体的业务场景说明 eventbus 比直接 RPC 更合适的原因。

---

#### 3-8 config — 动态配置热更新

**关键文件：**
- `config/config.go` — Configurator 接口
- `config/file/` — 文件实现（最简单，先读这个）
- `etc/` — 静态启动配置（与 config 的区别）

**学习重点：**
- Get / Set / Watch 方法
- `etc`（启动时一次性读取）与 `config`（运行时热更新）的区别
- 支持的格式：json / yaml / toml / xml

**提问模板：**
- "etc 和 config 各自适合存什么配置？"
- "config.Watch 怎么监听一个 key 的变化？"

**完成标志：** 能说出 etc 和 config 的职责边界，以及热更新的触发机制。

---

#### 3-9 cache / lock — 基础设施

**关键文件：**
- `cache/cache.go` — Cache 接口
- `lock/lock.go` — Locker 接口
- `cache/redis/` 和 `lock/redis/` — Redis 实现

**学习重点：**
- Cache：Get / Set / Del / Has / TTL
- Locker：Lock / Unlock / TryLock
- 分布式锁的典型使用场景

**提问模板：**
- "框架自身在哪些地方用到了 cache？"
- "分布式锁的超时和续期怎么处理？"

**完成标志：** 能说出 cache 和 lock 在框架内部的用途，而不仅仅是接口定义。

---

### 整体完成标志

能画出以下模块依赖关系图（方向表示"依赖"）：

```
cluster/gate ──→ session
             ──→ network
             ──→ registry
             ──→ locate
             ──→ internal/transporter

cluster/node ──→ registry
             ──→ locate
             ──→ transport (RPC)
             ──→ eventbus

internal/link──→ locate
             ──→ registry
             ──→ transport (内部协议)
```

---

## 第四步：动手练习 ⬜

每理解一个模块后，做一个小改动练习，不超过 20 行：

```
给我一个针对 lock 模块的小练习，不超过 20 行改动
```

**目标：** 通过修改代码验证理解，而不只是阅读。

**练习记录**

| 模块 | 练习内容 | 完成日期 |
|------|---------|----------|
| | | |

---

## 第五步：结合二次开发目标 ⬜

学习完核心模块后，针对自己的改动需求提问：

- 我想在 xxx 模块新增 yyy 功能，应该从哪里入手？
- 这个设计有什么扩展点？

**改动意向记录**
<!-- 在此记录二开目标，例如：新增某协议支持、扩展某注册中心等 -->

---

## 本地运行指南

### 环境要求

| 工具 | 版本 | 说明 |
|------|------|------|
| Go | 1.21+ | 编译运行 |
| Docker Desktop | 最新稳定版 | 运行中间件容器 |
| 公共基础设施 | — | `~/workspace/env/my-docker-config` |

---

### 第一步：一键启动中间件

本仓库提供了 `dev.sh` 脚本，封装了所有中间件的启动操作。

**前提：加载公共管理命令**（建议写入 `~/.zshrc`，只需配置一次）

```bash
echo 'source ~/workspace/env/my-docker-config/infra/scripts/infra.sh' >> ~/.zshrc
source ~/.zshrc
```

**启动**

```bash
# 默认启动：redis + consul + nats（推荐，覆盖 due 所有常用模块）
./dev.sh

# 用 etcd 替代 consul（适合测试 etcd 注册中心）
./dev.sh etcd

# 查看运行状态
./dev.sh status

# 停止所有 due 相关服务
./dev.sh stop
```

**due 框架各模块与服务的对应关系**

| 服务 | due 模块 | 端口 | Web UI |
|------|----------|------|--------|
| Redis | locate（用户位置）/ cache / lock | 6379 | — |
| Consul | registry（服务发现）/ config | 8500 | http://localhost:8500 |
| NATS | eventbus（跨节点事件） | 4222 | http://localhost:8222 |
| etcd | registry（可选，替代 consul） | 2379 | http://localhost:8070 |
| Nacos | registry / config（可选） | 8848 | http://localhost:8848/nacos |
| Kafka | eventbus（可选，替代 nats） | 9092 | — |

> Nacos 依赖 MySQL，需单独执行 `infra-nacos-up` 启动。

**启动后验证**

```bash
# 检查容器是否全部 running
./dev.sh status

# 验证 Redis 可访问
docker exec infra-redis redis-cli ping    # 应返回 PONG

# 验证 Consul 可访问
curl -s http://localhost:8500/v1/status/leader   # 应返回当前 leader 地址

# 验证 NATS 可访问
curl -s http://localhost:8222/healthz            # 应返回 OK
```

---

### 第二步：新建业务项目

due 是框架库，本仓库没有 `main.go`，需在外部新建项目引用：

```bash
mkdir ~/due-demo && cd ~/due-demo
go mod init due-demo

# 核心包
go get github.com/dobyte/due/v2@latest

# 按实际选用的组件添加（以下为 consul + redis + ws 默认组合）
go get github.com/dobyte/due/locate/redis/v2@latest
go get github.com/dobyte/due/network/ws/v2@latest
go get github.com/dobyte/due/registry/consul/v2@latest
```

> 子模块版本须与主模块对齐，版本不一致时执行：
> `go get github.com/dobyte/due/locate/redis/v2@<主模块commit>`

---

### 第三步：启动 Gate

```go
// gate/main.go
package main

import (
    "github.com/dobyte/due/locate/redis/v2"
    "github.com/dobyte/due/network/ws/v2"
    "github.com/dobyte/due/registry/consul/v2"
    due "github.com/dobyte/due/v2"
    "github.com/dobyte/due/v2/cluster/gate"
)

func main() {
    container := due.NewContainer()
    container.Add(gate.NewGate(
        gate.WithServer(ws.NewServer()),       // WS 服务器，监听 :3553
        gate.WithLocator(redis.NewLocator()),  // 用户位置追踪
        gate.WithRegistry(consul.NewRegistry()), // 服务注册
    ))
    container.Serve()
}
```

```bash
cd gate && go run main.go
```

启动成功日志示例：

```
┌─────────────────────────Gate─────────────────────────┐
| Name: gate                                           |
| Server: [ws] 0.0.0.0:3553                            |
| Locator: redis                                       |
| Registry: consul                                     |
└──────────────────────────────────────────────────────┘
```

---

### 第四步：启动 Node

```go
// node/main.go
package main

import (
    "fmt"
    "github.com/dobyte/due/locate/redis/v2"
    "github.com/dobyte/due/registry/consul/v2"
    due "github.com/dobyte/due/v2"
    "github.com/dobyte/due/v2/cluster/node"
    "github.com/dobyte/due/v2/codes"
    "github.com/dobyte/due/v2/log"
    "github.com/dobyte/due/v2/utils/xtime"
)

const routeGreet = 1

func main() {
    container := due.NewContainer()
    component := node.NewNode(
        node.WithLocator(redis.NewLocator()),
        node.WithRegistry(consul.NewRegistry()),
    )
    component.Proxy().Router().AddRouteHandler(routeGreet, false, greetHandler)
    container.Add(component)
    container.Serve()
}

type greetReq struct{ Message string `json:"message"` }
type greetRes struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}

func greetHandler(ctx node.Context) {
    req := &greetReq{}
    res := &greetRes{}
    defer ctx.Response(res)

    if err := ctx.Parse(req); err != nil {
        log.Errorf("parse failed: %v", err)
        res.Code = codes.InternalError.Code()
        return
    }
    res.Code = codes.OK.Code()
    res.Message = fmt.Sprintf("server time: %s", xtime.Now().Format(xtime.DateTime))
}
```

```bash
cd node && go run main.go
```

启动成功日志示例：

```
┌─────────────────────────Node─────────────────────────┐
| Name: node                                           |
| Locator: redis                                       |
| Registry: consul                                     |
└──────────────────────────────────────────────────────┘
```

---

### 常见问题

**中间件启动失败**
```bash
# 查看具体容器日志
infra-logs redis
infra-logs consul
infra-logs nats
```

**端口冲突**
```bash
# 查看占用端口的进程
lsof -i :6379
lsof -i :8500
lsof -i :4222
```

**子模块版本不一致**

参见 README.md 第 19 节「常见问题」，或执行：
```bash
go get github.com/dobyte/due/<sub-module>/v2@<commit-hash>
```

---

### 单模块测试（无需启动集群）

```bash
# packet / session 无外部依赖，直接运行
go test github.com/dobyte/due/v2/packet -v
go test github.com/dobyte/due/v2/session -v

# locate 需要 Redis（先执行 ./dev.sh）
go test github.com/dobyte/due/v2/locate/redis -v
```

---

## 二次开发指南

### 扩展点总览

due 的所有核心模块都以接口形式对外暴露，替换任意一层只需实现对应接口并在组件初始化时注入。

| 扩展点 | 接口文件 | 注入位置 |
|--------|----------|----------|
| 网络协议（tcp/kcp/ws） | `network/server.go` | `gate.WithServer()` |
| 服务注册中心 | `registry/registry.go` | `gate.WithRegistry()` / `node.WithRegistry()` |
| 用户位置追踪 | `locate/locator.go` | `gate.WithLocator()` / `node.WithLocator()` |
| 节点间 RPC | `transport/transporter.go` | `node.WithTransporter()` / `mesh.WithTransporter()` |
| 事件总线 | `eventbus/eventbus.go` | `eventbus.SetEventbus()` |
| 动态配置 | `config/config.go` | `config.SetConfigurator()` |
| 分布式锁 | `lock/lock.go` | `lock.SetLocker()` |
| 缓存 | `cache/cache.go` | `cache.SetCacher()` |

### 场景一：新增路由 Handler

业务逻辑全部写在 Node 的 RouteHandler 里，这是最常见的扩展点：

```go
// 定义路由常量（与客户端约定）
const (
    RouteLogin  = 1001
    RouteLogout = 1002
    RouteBattle = 2001
)

func initListen(proxy *node.Proxy) {
    r := proxy.Router()
    r.AddRouteHandler(RouteLogin,  false, loginHandler)
    r.AddRouteHandler(RouteLogout, false, logoutHandler)
    r.AddRouteHandler(RouteBattle, true,  battleHandler) // true = 需要登录态
}
```

**关键文件：** `cluster/node/router.go`，`AddRouteHandler` 第三个参数 `stateful bool` 控制是否要求已绑定 UID。

### 场景二：使用 Actor 隔离有状态逻辑

适合房间、战斗场景等需要独立状态的业务单元，避免跨 goroutine 的锁竞争：

```go
// 实现 node.Processor 接口
type RoomActor struct {
    node.Actor
    players map[int64]string
}

func (r *RoomActor) Init() {
    r.players = make(map[int64]string)
}

func (r *RoomActor) Destroy() {}

// 在 Node 启动后通过 Proxy 创建 Actor
proxy.Actor().Create(ctx, &RoomActor{})
```

**关键文件：** `cluster/node/actor.go`，`cluster/node/proxy.go`（Actor 方法）。

### 场景三：Node 推送消息给客户端

Node 不直接持有连接，必须经 GateLinker → Gate → session → Conn 这条路径：

```go
// 推送给单个用户（按 UID）
proxy.Push(ctx, &node.PushArgs{
    UID:     uid,
    Message: &cluster.Message{Route: 3001, Data: data},
})

// 广播给所有在线用户
proxy.Broadcast(ctx, &node.BroadcastArgs{
    Kind:    cluster.User,
    Message: &cluster.Message{Route: 3001, Data: data},
})
```

**关键文件：** `cluster/node/proxy.go`，`internal/link/gate.go`。

### 场景四：Node 调用 Mesh 微服务

Mesh 适合无状态的计算服务（排行榜、匹配服等），Node 通过 transport RPC 调用：

```go
// node handler 内调用 mesh
func battleHandler(ctx node.Context) {
    res := &MatchResult{}
    if err := ctx.CallNode(RouteMeshMatch, req, res); err != nil {
        // 处理错误
    }
}
```

**关键文件：** `internal/link/node.go`（doRPC），`transport/transporter.go`（Client 接口）。

### 场景五：实现自定义注册中心

若现有的 consul/etcd/nacos 都不满足需求，可以自己实现：

```go
// 实现 registry.Registry 接口（registry/registry.go）
type MyRegistry struct{}

func (r *MyRegistry) Register(ctx context.Context, ins *registry.ServiceInstance) error   { ... }
func (r *MyRegistry) Deregister(ctx context.Context, ins *registry.ServiceInstance) error { ... }
func (r *MyRegistry) Watch(ctx context.Context, name string) (registry.Watcher, error)    { ... }
func (r *MyRegistry) Services(ctx context.Context, name string) ([]*registry.ServiceInstance, error) { ... }

// 注入
gate.WithRegistry(&MyRegistry{})
```

**关键文件：** `registry/registry.go`（接口定义），参考 `registry/consul/` 的实现。

### 场景六：跨节点事件通知

玩家上线/下线、全服公告等无需返回值的广播，用 eventbus 而不是 RPC：

```go
// 发布事件（Gate 或 Node 均可）
eventbus.Publish(ctx, "player.online", &PlayerOnlineEvent{UID: uid})

// 订阅事件（任意节点）
eventbus.Subscribe(ctx, "player.online", func(event *eventbus.Event) {
    payload := &PlayerOnlineEvent{}
    event.Decode(payload)
    // 处理逻辑
})
```

**关键文件：** `eventbus/eventbus.go`，实现参考 `eventbus/redis/`。

### 二开原则

1. **只动 Proxy，不动内部** — 业务代码只通过 `gate.Proxy` / `node.Proxy` 交互，不直接访问 session、linker 等内部结构。
2. **接口替换，不 fork 核心** — 换注册中心、换网络层，实现接口注入即可，不要修改框架核心文件。
3. **路由常量集中管理** — 所有路由号放在一个包里统一定义，客户端和服务端共用同一份。
4. **有状态用 Actor，无状态用 Mesh** — 避免在普通 Handler 里用全局锁保护共享状态。

---

## 新人源码阅读路线

面向第一次接触 due 的开发者，按照以下顺序阅读，每一步都建立在前一步的基础上。

### 第一阶段：搞清楚框架是什么（1 天）

目标：能用一句话说清楚 due 解决什么问题，四个节点角色各干什么。

| 顺序 | 文件 | 读什么 |
|------|------|--------|
| 1 | `README.md` | 整体介绍、架构图、Gate/Node/Mesh 区别 |
| 2 | `container.go` | `NewContainer` / `Add` / `Serve`，理解生命周期 |
| 3 | `component/component.go` | `Component` 接口定义，只有 4 个方法 |
| 4 | `cluster/gate/gate.go` 前 50 行 | Gate 的字段组成，知道它持有什么 |
| 5 | `cluster/node/node.go` 前 50 行 | Node 的字段组成，和 Gate 对比 |

**检验方式：** 不看代码，能画出 Container 与 Gate/Node 的关系图。

---

### 第二阶段：跟踪一条消息（2 天）

目标：能独立描述客户端发一条消息到 Node Handler 的完整路径，说出每一跳的文件名和函数名。

**上行路径（客户端 → Node）：**

```
network/tcp/server.go       → Start() 接受连接，回调 OnReceive
cluster/gate/gate.go        → handleReceive()  收到原始字节
cluster/gate/proxy.go       → deliver()        解包 + 定位 Node
internal/link/node.go       → Deliver()        通过 transporter 转发
cluster/node/router.go      → deliver() / handle()  分发到 Handler
```

**下行路径（Node → 客户端）：**

```
cluster/node/proxy.go       → Push()
internal/link/gate.go       → Push()  定位 UID 所在 Gate
internal/transporter/gate/  → client.Push() / server.push()
session/session.go          → Push()  找到 Conn
network/tcp/server_conn.go  → Push()  写入 TCP 缓冲区
```

**读代码时的辅助方法：**
```bash
# 查找某函数的所有调用方
grep -rn "deliver(" cluster/ internal/ --include="*.go"

# 查找接口的所有实现
grep -rn "func.*Deliver(" . --include="*.go"
```

---

### 第三阶段：理解关键设计决策（2 天）

每读完一个问题，用自己的话回答，再让 Claude 纠正。

| 问题 | 相关文件 |
|------|----------|
| CID 和 UID 的区别，什么时候绑定 UID？ | `session/session.go` |
| registry 和 locate 分别追踪什么？ | `registry/registry.go`，`locate/locator.go` |
| internal/transporter 和 transport 的区别？ | `internal/transporter/`，`transport/transporter.go` |
| Actor 为什么能不加锁处理消息？ | `cluster/node/actor.go` |
| Node 为什么不能直接写客户端连接？ | `cluster/node/proxy.go` Push 方法 |
| heartbeat 包和数据包在 header 上怎么区分？ | `packet/packet.go` |

---

### 第四阶段：跑通完整链路（1 天）

参考「本地运行指南」章节，在本地跑起来 Gate + Node + Client，观察日志输出，对照第二阶段的调用链验证理解。

---

## 调试工具指南

due 的通信协议分两类，调试工具的选择取决于你调的是哪一层：

| 层 | 协议 | 适合工具 |
|----|------|----------|
| 客户端 ↔ Gate | TCP / KCP / WebSocket + due packet 二进制协议 | 内置 `cluster/client`、`network/tcp` 测试客户端 |
| HTTP 管理接口 | HTTP/HTTPS（fiber） | **Bruno** ✅ |
| Node ↔ Mesh | gRPC / rpcx | grpcurl、BloomRPC |

### Bruno 的适用范围

Bruno 是 HTTP API 调试工具。due 框架在 `component/http` 提供了基于 fiber 的 HTTP 服务器组件，**只有业务方挂载了 HTTP 组件时，才有接口可以用 Bruno 调试**。

典型使用场景：
- 后台管理 API（查玩家数据、踢人、发公告）
- 运营工具接口
- Mesh 服务如果对外暴露了 HTTP 端点

**HTTP 组件用法：**

```go
import httpcomp "github.com/dobyte/due/component/http/v2"

server := httpcomp.NewServer(
    httpcomp.WithAddr(":8080"),
)

server.Proxy().Router().
    Post("/api/kick", kickHandler).
    Get("/api/player/:uid", getPlayerHandler)

container.Add(server)
```

启动后，就可以在 Bruno 中创建 Collection，新建请求：

```
POST http://localhost:8080/api/kick
Content-Type: application/json

{
  "uid": 10001,
  "reason": "违规"
}
```

### Bruno 项目结构建议

在业务项目根目录建一个 `bruno/` 文件夹：

```
your-game-server/
├── bruno/
│   ├── bruno.json          ← Collection 配置
│   ├── environments/
│   │   ├── local.bru       ← 本地环境变量（baseUrl、token 等）
│   │   └── staging.bru
│   ├── admin/
│   │   ├── kick.bru
│   │   └── player-info.bru
│   └── mesh/
│       └── match.bru
```

`bruno.json` 示例：
```json
{
  "version": "1",
  "name": "game-server",
  "type": "collection"
}
```

`environments/local.bru` 示例：
```
vars {
  baseUrl: http://localhost:8080
  adminToken: dev-token-123
}
```

请求文件 `admin/kick.bru` 示例：
```
meta {
  name: 踢出玩家
  type: http
  seq: 1
}

post {
  url: {{baseUrl}}/api/kick
  body: json
  auth: none
}

headers {
  Authorization: Bearer {{adminToken}}
}

body:json {
  {
    "uid": 10001,
    "reason": "违规操作"
  }
}
```

### 调试游戏协议（TCP/WS 路由消息）

游戏主协议（Gate ↔ Node）是二进制的 due packet，**Bruno 不支持**。有两种方式调试：

**方式一：使用框架内置 cluster/client（推荐）**

框架自带测试客户端，直接写 Go 代码模拟客户端行为：

```go
// client/main.go
package main

import (
    due "github.com/dobyte/due/v2"
    "github.com/dobyte/due/v2/cluster"
    "github.com/dobyte/due/v2/cluster/client"
    "github.com/dobyte/due/network/ws/v2"
)

func main() {
    container := due.NewContainer()
    component := client.NewClient(
        client.WithClient(ws.NewClient()),
    )

    proxy := component.Proxy()
    proxy.AddHookListener(cluster.Start, func(p *client.Proxy) {
        conn, _ := p.Dial()
        _ = conn.Push(&cluster.Message{
            Route: 1001,
            Data:  []byte(`{"token":"test123"}`),
        })
    })
    proxy.AddRouteHandler(1001, func(ctx *client.Context) {
        // 打印服务端响应
    })

    container.Add(component)
    container.Serve()
}
```

**方式二：直接用 network/tcp 层测试（更底层）**

参考 `network/tcp/client_test.go`，手动 Pack/Unpack packet，适合测试协议编解码本身：

```go
conn, _ := tcp.NewClient().Dial()
msg, _ := packet.PackMessage(&packet.Message{
    Seq:    1,
    Route:  1001,
    Buffer: []byte(`{"token":"test123"}`),
})
conn.Push(msg)
```

### 调试 Mesh gRPC 接口

如果 Mesh 使用 gRPC transport，可以用 `grpcurl` 命令行工具：

```bash
# 列出所有服务
grpcurl -plaintext localhost:9000 list

# 调用接口
grpcurl -plaintext -d '{"uid": 10001}' localhost:9000 MatchService/Match
```

---

## 常用提问模式

| 场景 | 提问方式 |
|------|----------|
| 找实现位置 | "eventbus 的发布订阅在哪里实现的？" |
| 读懂代码片段 | 贴代码，问"这段在做什么，为什么这样设计" |
| 了解模块边界 | "哪些模块会调用 session？依赖关系是什么" |
| 追踪数据流 | "一条消息从客户端进来，经过哪些模块" |
| 验证理解 | "我理解 xxx 是这样工作的，对吗？" |
