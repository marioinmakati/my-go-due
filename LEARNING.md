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

## 常用提问模式

| 场景 | 提问方式 |
|------|----------|
| 找实现位置 | "eventbus 的发布订阅在哪里实现的？" |
| 读懂代码片段 | 贴代码，问"这段在做什么，为什么这样设计" |
| 了解模块边界 | "哪些模块会调用 session？依赖关系是什么" |
| 追踪数据流 | "一条消息从客户端进来，经过哪些模块" |
| 验证理解 | "我理解 xxx 是这样工作的，对吗？" |
