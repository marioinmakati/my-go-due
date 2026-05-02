# 文档导航

本仓库共有以下几份文档，按照下面的顺序阅读效果最好。

---

## 推荐阅读顺序

### 第一步：了解框架本身
**[README.md](README.md)**

due 框架的官方介绍。包含：框架是什么、四种节点角色（Gate/Node/Mesh/Client）、消息生命周期、通信协议格式、快速启动示例、二次开发指南。

> 新人从这里开始。读完能回答"due 解决什么问题、Gate 和 Node 分别做什么"。

---

### 第二步：制定学习计划，逐步深入源码
**[LEARNING.md](LEARNING.md)**

源码学习的完整路线图。包含：

| 章节 | 内容 |
|------|------|
| 第一步～第三步 | Container 生命周期 → 消息流 → 各模块深入，共 9 个模块 |
| 新人源码阅读路线 | 4 个阶段、每阶段的阅读顺序和检验方式 |
| 本地运行指南 | 启动中间件、Gate、Node 的完整命令 |
| 调试工具指南 | Bruno 适用范围、cluster/client 调试游戏协议、grpcurl 调试 Mesh |
| 二次开发指南 | 扩展点总览 + 6 个业务场景代码示例 |

> 源码学习的主力文档，按章节顺序推进。

---

### 第三步：结合业务目标，规划棋牌开发
**[CHESS_LEARNING.md](CHESS_LEARNING.md)**

棋牌游戏开发学习路线。结合本仓库（due 框架）和业务仓库（my-go-chess），从零构建棋牌游戏的完整知识体系。

> 有具体业务目标时再看，框架源码读完后阅读效果更好。

---

### 随时可查：工具文档

**[UPSTREAM.md](UPSTREAM.md)**

上游同步与本地补丁管理。说明如何从 `dobyte/due` 拉取上游更新、解决冲突、管理本地补丁分支。

> 需要同步上游代码时查阅。

**[CLAUDE.md](CLAUDE.md)**

Claude Code 的项目级指令，供 AI 辅助工具使用，开发者一般不需要直接阅读。

---

## 本地开发环境快速启动

中间件统一使用公共 Docker 配置，详细说明见 [LEARNING.md — 本地运行指南](LEARNING.md#本地运行指南)。

```bash
# 一键启动 due 所需中间件（redis + consul + nats）
./dev.sh

# 停止
./dev.sh stop

# 查看状态
./dev.sh status
```

---

## 一句话总结

```
README.md         → 框架是什么
LEARNING.md       → 怎么读源码 + 怎么本地跑 + 怎么二次开发
CHESS_LEARNING.md → 棋牌业务怎么做
UPSTREAM.md       → 怎么跟上游同步
```
