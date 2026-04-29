# 上游同步与本地补丁管理

本仓库基于 [dobyte/due](https://github.com/dobyte/due) 进行二次开发，只拉取上游更新，不向上游推送。

## 远端配置

```bash
# 首次设置：添加上游远端
git remote add upstream https://github.com/dobyte/due.git

# 验证
git remote -v
# origin    <你的私有仓库URL> (fetch)
# upstream  https://github.com/dobyte/due.git (fetch)
```

## 分支结构

```
upstream/main  ──A──B──C──D
                          │
local-patches  ──────────P1──P2──P3──...
```

- `upstream/main`：跟踪上游，只读，不做任何改动
- `local-patches`：所有本地二次开发的改动都在此分支上

## 初次建立本地补丁分支

```bash
git fetch upstream
git checkout -b local-patches upstream/main
# 在此分支上进行所有本地开发
```

## 拉取上游更新

```bash
git fetch upstream
git rebase upstream/main local-patches
```

rebase 过程中遇到冲突：

```bash
# 解决冲突后
git add <冲突文件>
git rebase --continue

# 若某个本地 commit 已被上游吸收，跳过
git rebase --skip
```

## 本地改动规范

所有本地 commit 使用 `[local]` 前缀，便于与上游提交区分，也方便 rebase 时识别：

```bash
git commit -m "[local] 调整默认配置"
git commit -m "[local] 新增业务模块 xxx"
```

## 推送到自己的私有仓库

```bash
# rebase 后需要强制推送到自己的 origin
git push origin local-patches --force-with-lease
```
