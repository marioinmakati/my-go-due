# 棋牌游戏开发学习路线

结合 `my-go-chess`（具体业务实现）和 `my-go-due`（分布式框架）两个项目，从零构建棋牌游戏开发的完整知识体系。

**两个项目的角色分工：**
- `my-go-chess` → 读懂"棋牌游戏怎么做"，理解业务模型和并发设计
- `my-go-due` → 读懂"分布式游戏服务器怎么架构"，理解工业级框架设计

---

## 阶段一：连接与协议（2 天）

> 目标：弄清楚一个客户端连进来，服务器做了什么。

### chess 中读这些

| 文件 | 关注点 |
|------|--------|
| `core/protocol/protocol.go` | 二进制帧格式：4 字节大端长度前缀 + body |
| `core/network/conn.go` | Conn 封装：ID 分配、读写、关闭 |
| `server/network/` | TCP / WebSocket 监听，连接建立后调用 `database.Connected()` |
| `server/database/model.go:102` | `player.Listening()`：独立 goroutine 读包写入 `p.data` channel |

### due 中对比读这些

| 文件 | 关注点 |
|------|--------|
| `transport/tcp/`、`transport/ws/` | 框架如何封装相同的 TCP/WS 监听 |
| `packet/packet.go` | due 的包格式：size+header+route+seq+data，对比 chess 的帧格式有何不同 |
| `core/net/` | 框架底层连接抽象 |

### 验证理解

- chess 的包格式和 due 的包格式各有什么字段？多出来的 `route` 和 `seq` 解决了什么问题？
- `player.Listening()` 为什么要用独立 goroutine + channel，而不是在主循环里直接 read？

---

## 阶段二：玩家生命周期与状态机（2 天）

> 目标：一个玩家从连接到进入游戏房间的完整流程。

### chess 中读这些

| 文件 | 关注点 |
|------|--------|
| `server/state/state.go:40` | `Run()` 主循环：`for { state.Next(player) }` |
| `server/state/welcome.go` | 第一个状态：认证 |
| `server/state/home.go` | 菜单：加入房间 or 创建房间 |
| `server/state/create.go` | 建房 → 返回 `StateWaiting` |
| `server/state/waiting.go:130` | `startGame()` 分派到各游戏 `Init` |
| `server/database/model.go:233` | `player.State()` / `player.GetState()` 原子操作 |

**核心问题：**
- `state.Next()` 返回的是下一个 StateID，谁负责切换？
- 两个 goroutine（`Listening` 读包 + `Run` 消费）如何通过 `p.data` channel 协作，又如何在断线时安全关闭？
- `AskForPacket` 阻塞在哪里？超时怎么处理？

### due 中对比读这些

| 文件 | 关注点 |
|------|--------|
| `cluster/gate/gate.go` | Gate 的生命周期：Init → Start → Close |
| `session/session.go` | 框架如何管理 CID→Conn / UID→Conn 的映射，对比 chess 的内存 hashmap |
| `cluster/node/node.go` | Node 的 `fnChan`、`router`、`trigger` 各自负责什么 |
| `component/component.go` | 框架统一生命周期接口 |

### 验证理解

- chess 是"每个玩家一个状态机 goroutine"，due 是"Router 分发 + Handler"，各有什么优缺点？
- chess 的 `Room` 和 due 的 `session.Session` 都在管理连接集合，设计思路有何不同？

---

## 阶段三：斗地主——第一个完整游戏（3 天）

> 目标：读懂一局游戏从发牌到结算的完整代码，理解棋牌游戏的并发模型。

### chess 中读这些（按顺序）

**Day 1：发牌与抢庄**

| 文件:行号 | 关注点 |
|-----------|--------|
| `server/database/model.go:299` | `Game` 结构：Players、Cards、Landlord、States |
| `server/state/game/game.go:342` | `InitGame()`：洗牌、发牌、确定底牌 |
| `server/state/game/game.go:31` | `Next()` 主循环：`stateRob → statePlay` |
| `server/state/game/game.go:102` | `handleRob()`：抢庄协议，三个玩家通过 `States map[int64]chan int` 轮流发信号 |

**Day 2：出牌与胜负**

| 文件:行号 | 关注点 |
|-----------|--------|
| `server/state/game/game.go:181` | `playing()`：出牌核心循环 |
| `core/util/poker/` | `ParseFaces` / `Compare`：手牌识别与大小比较 |
| `server/rule/` | 通用出牌规则校验 |

**Day 3：变种对比**

在 `game.go` 里 grep `EnableLaiZi` / `EnableSkill`，列出每个分支位置，理解"同文件多 flag"的代价。

**核心问题：**
- `game.States[playerID] <- statePlay` 如何让另一个 goroutine 醒来？三个 goroutine 如何通过 channel 无锁轮换出牌？
- 底牌揭示的时机是在抢庄结束后，还是发牌时？代码在哪里？

---

## 阶段四：德州扑克——有状态节点的典型场景（3 天）

> 目标：理解多街游戏、边池计算、断线重连三个难点。

### chess 中读这些（按顺序）

| 文件 | 关注点 |
|------|--------|
| `server/state/game/texas/init.go` | 座位分配、盲注收取 |
| `server/state/game/texas/texas.go` | `Next()` 中 `select + time.After(60s)`：断线超时自动 fold |
| `server/state/game/texas/round.go` | 四街：preflop → flop → turn → river |
| `server/state/game/texas/bet.go` | `BetRound()`：call / raise / fold / allin |
| `server/database/texas.go` | `Texas.Bet()`、`RoundEnd()`、边池分配逻辑 |
| `server/database/texas_test.go` | 14 个测试用例，直接运行验证理解 |

**核心问题：**
- `TexasPlayer.State chan int` 为何不复用斗地主的 `Game.States map[int64]chan int`？
- allin 后边池（side pot）如何计算？谁赢谁分？
- 断线重连：`Offline()` 在游戏进行时为何不关闭 `data` channel？`Connected()` 调用的是 `Reconnect()` 而非新建 Player，差异在哪？

### due 中对比读这些

| 文件 | 关注点 |
|------|--------|
| `locate/locator.go` | 框架如何追踪 UID 在哪个 Gate / 哪个 Node |
| `cluster/node/actor.go` | Actor 模型：有状态节点的另一种封装方式 |
| `cluster/node/timer.go` | 框架的定时器，对比 chess 里 `time.After(60s)` 的裸用法 |

**对比问题：**
- chess 的断线重连是"玩家对象保留在内存"，due 的方案是"locate 追踪 + Gate 重建 session"，各有什么局限？
- 如果要做多节点德州扑克（一张桌子跨两台机器），due 的哪些模块是必须用到的？

---

## 阶段五：框架层深入——due 的分布式机制（3 天）

> 目标：理解 due 框架如何把多个 Gate + Node 组织成集群，为后续扩展 chess 打基础。

### 读这些（按顺序）

**Day 1：消息路由**

| 文件 | 关注点 |
|------|--------|
| `cluster/gate/gate.go` | Gate 收到客户端消息后如何转发给 Node |
| `internal/transporter/` | Gate ↔ Node 私有通信协议（区别于对外的 transport RPC） |
| `cluster/node/router.go` | Node 的路由表：route ID → Handler 函数 |
| `cluster/node/proxy.go` | Node 通过 Proxy 向 Gate 推消息（Push / Multicast / Broadcast） |

**Day 2：服务发现与用户定位**

| 文件 | 关注点 |
|------|--------|
| `registry/registry.go` | 服务注册/发现接口 |
| `locate/locator.go` | UID → GateID / NodeID 的追踪接口 |
| `cluster/cluster.go` | 节点状态：Work / Busy / Hang / Shut |

**Day 3：事件总线与 Actor**

| 文件 | 关注点 |
|------|--------|
| `eventbus/eventbus.go` | 跨节点异步事件：Connect / Disconnect / 自定义事件 |
| `cluster/node/actor.go` | Actor 模型：每个 Actor 独占消息队列，无锁处理有状态逻辑 |
| `cluster/node/scheduler.go` | 定时任务调度 |

### 验证理解

- chess 里广播一条消息：`room.Broadcast(msg)` 直接遍历内存 map。due 里广播：`proxy.Broadcast()` 需要经过哪些层？
- due 的 `Actor` 和 chess 的"每个 goroutine 一个玩家"本质上解决的是同一个问题，各有什么取舍？

---

## 阶段六：整合视角——如何用 due 重构 chess（1 天）

> 目标：不是真的重构，而是做一次纸上设计，建立"框架 vs 手写"的完整对照。

### 设计练习

画出用 due 实现 chess 的模块映射：

| chess 现有组件 | due 对应替换 |
|----------------|-------------|
| `server/network/` TCP/WS 监听 | `cluster/gate` + `transport/tcp` / `transport/ws` |
| `server/state/state.go` 状态机 | `cluster/node/router.go` Route → Handler |
| `server/database/` 内存 hashmap | `session.Session` (Gate 侧) + `locate` (跨节点追踪) |
| `room.Broadcast()` 遍历内存 | `proxy.Broadcast()` 经由 Gate 转发 |
| `time.After(60s)` 裸超时 | `cluster/node/scheduler.go` 定时器 |
| 无（单节点） | `registry` 服务发现 + `eventbus` 跨节点事件 |
| 无（单节点） | `cluster/node/actor.go` 有状态房间封装 |

**思考题：** chess 里德州扑克的 `texas/texas.go:Next()` 是一个阻塞的游戏主循环。迁移到 due 后，这个循环应该放在 Node 的什么地方？用 Actor 还是普通 Handler？

---

## 时间汇总

| 阶段 | 内容 | 建议时间 |
|------|------|---------|
| 一 | 连接与协议 | 2 天 |
| 二 | 玩家生命周期与状态机 | 2 天 |
| 三 | 斗地主（第一个完整游戏） | 3 天 |
| 四 | 德州扑克（断线重连、边池） | 3 天 |
| 五 | due 框架分布式机制 | 3 天 |
| 六 | 整合视角：纸上重构设计 | 1 天 |
| **合计** | | **约 14 天** |

---

## 关键问题清单（学完可作为自测）

1. chess 的"两个 goroutine per player"模型是什么？`Listening` 和 `Run` 各自做什么？channel 关闭时序是？
2. 斗地主三个玩家如何通过 channel 无锁轮流出牌？
3. 德州扑克边池怎么算？代码在哪里？
4. 断线重连：chess 和 due 各自的方案是什么？各有什么局限？
5. due 的 Gate 和 Node 之间用什么通信？和对外暴露的 `transport` 接口是同一套吗？
6. due 的 `locate` 解决了什么问题？chess 为什么不需要它？
7. Actor 模型和"每个 goroutine 一个玩家"本质上解决的是同一个问题吗？取舍是什么？
8. 如果要把 chess 做成水平可扩展，最小改动是什么？
