# Stash - 暂存工作区修改

`zeta stash` 命令用于暂存工作区和索引的修改，以便在不提交的情况下切换分支或执行其他操作。这对于需要临时保存工作进度的场景非常有用。

## 一、基本概念

### 1.1 什么是 Stash

Stash 是一个栈结构，用于临时保存工作区和索引的修改状态。当你需要：

- 切换分支但不想提交当前修改
- 暂时处理其他紧急任务
- 在不同分支间共享修改

可以使用 stash 保存当前工作状态。

### 1.2 Stash 的结构

在 HugeSCM 中，stash 采用类似 Git 的存储策略：

```
stash 存储结构：
┌─────────────────────────────────────┐
│  Stash Entry (stash@{0})           │
├─────────────────────────────────────┤
│  Index Commit (A)                   │  ← 暂存区的状态
│  - parents: [HEAD]                  │
│  - tree: index tree                 │
├─────────────────────────────────────┤
│  Worktree Commit (B)                │  ← 工作区的状态
│  - parents: [Index Commit, HEAD]    │
│  - tree: worktree tree              │
└─────────────────────────────────────┘
```

**工作原理：**

1. 将 index 创建一个提交 A，A 的 parents 为 HEAD，其 tree 为 index 的 tree
2. 创建一个合并提交 B，其父提交是 A 和 HEAD，其 tree 为 worktree 的 tree

这种设计允许 stash 在恢复时正确处理 index 和 worktree 的差异。

## 二、基本用法

### 2.1 创建 Stash

```bash
# 暂存所有修改（工作区 + 暂存区）
zeta stash

# 带描述信息
zeta stash save "WIP: 用户认证功能"

# 仅暂存已跟踪文件的修改
zeta stash --keep-index

# 包含未跟踪的文件
zeta stash --include-untracked

# 包含未跟踪和忽略的文件
zeta stash --all
```

### 2.2 查看 Stash 列表

```bash
# 列出所有 stash
zeta stash list

# 输出示例：
# stash@{0}: On mainline: WIP: 用户认证功能
# stash@{1}: WIP on feature: 数据导入优化
# stash@{2}: On mainline: 临时保存
```

### 2.3 查看 Stash 详情

```bash
# 查看 stash 的详细变更
zeta stash show stash@{0}

# 查看完整 diff
zeta stash show -p stash@{0}
```

### 2.4 应用 Stash

```bash
# 应用最近的 stash（不删除）
zeta stash apply

# 应用指定的 stash
zeta stash apply stash@{2}

# 应用并从列表中删除
zeta stash pop

# 应用指定的 stash 并删除
zeta stash pop stash@{2}
```

### 2.5 删除 Stash

```bash
# 删除指定的 stash
zeta stash drop stash@{0}

# 删除所有 stash
zeta stash clear
```

## 三、命令选项

### 3.1 stash save 选项

| 选项 | 说明 |
|-----|------|
| `-p, --patch` | 交互式选择要暂存的修改 |
| `-k, --keep-index` | 保持暂存区不变 |
| `-u, --include-untracked` | 包含未跟踪文件 |
| `-a, --all` | 包含未跟踪和忽略的文件 |
| `-m, --message <msg>` | 添加描述信息 |

### 3.2 stash apply/pop 选项

| 选项 | 说明 |
|-----|------|
| `--index` | 恢复暂存区状态 |

## 四、Stash 恢复流程

### 4.1 正常恢复

当 HEAD 未改变时，stash 可以完美恢复：

```
保存时状态:
HEAD: commit A
index: 修改 X
worktree: 修改 X + 修改 Y

恢复后:
HEAD: commit A (未变)
index: 修改 X
worktree: 修改 X + 修改 Y
```

### 4.2 HEAD 改变后的恢复

如果 HEAD 在保存 stash 后发生了变化：

```
保存时:
HEAD: commit A
stash: 修改 X

切换分支后:
HEAD: commit B

恢复 stash:
尝试合并修改 X 到 commit B
- 无冲突：成功恢复
- 有冲突：需要手动解决
```

### 4.3 冲突处理

当 stash pop/apply 产生冲突时：

```
$ zeta stash pop
错误：stash 恢复时产生冲突
CONFLICT (content): Merge conflict in src/auth.go

# 解决冲突
# 编辑冲突文件...

# 标记冲突已解决
zeta add src/auth.go

# stash 会自动从列表中移除（pop 时）
# 或手动删除（apply 时）
zeta stash drop
```

### 4.4 恢复暂存区状态

默认情况下，`stash apply` 不会恢复暂存区状态。使用 `--index` 选项：

```bash
# 同时恢复暂存区状态
zeta stash apply --index

# 如果 HEAD 改变，暂存区恢复可能失败
# 此时可以先恢复工作区，再手动 add
```

## 五、使用场景

### 5.1 临时切换分支

```bash
# 场景：在 feature 分支工作，需要紧急修复 mainline 的 bug

# 保存当前工作
zeta stash save "WIP: 功能开发中"

# 切换到 mainline
zeta switch mainline
zeta pull

# 修复 bug
zeta add .
zeta commit -m "fix: 紧急修复 XXX 问题"
zeta push

# 返回 feature 分支
zeta switch feature

# 恢复工作
zeta stash pop
```

### 5.2 暂存部分修改

```bash
# 场景：只想暂存部分文件

# 使用 --patch 交互式选择
zeta stash save --patch

# 或先 add 想保留的文件，再 stash --keep-index
zeta add file-to-keep.c
zeta stash --keep-index
```

### 5.3 保留未跟踪文件

```bash
# 场景：创建了新文件但还不想提交

# 默认 stash 不包含新文件
zeta stash                    # 新文件不会被暂存

# 使用 --include-untracked
zeta stash --include-untracked  # 新文件也会被暂存
```

### 5.4 多个 Stash 管理

```bash
# 创建多个 stash
zeta stash save "功能 A 开发中"
zeta stash save "功能 B 实验性修改"

# 查看列表
zeta stash list

# 应用特定的 stash
zeta stash apply stash@{1}
```

## 六、与 Git 的兼容性

HugeSCM 的 stash 功能与 Git 基本兼容：

| Git 命令 | HugeSCM 命令 | 说明 |
|---------|-------------|------|
| `git stash` | `zeta stash` | 功能相同 |
| `git stash list` | `zeta stash list` | 功能相同 |
| `git stash pop` | `zeta stash pop` | 功能相同 |
| `git stash apply` | `zeta stash apply` | 功能相同 |
| `git stash drop` | `zeta stash drop` | 功能相同 |
| `git stash clear` | `zeta stash clear` | 功能相同 |

## 七、最佳实践

### 7.1 使用描述性消息

```bash
# 不推荐
zeta stash

# 推荐
zeta stash save "WIP: 用户认证模块，缺少密码验证"
```

### 7.2 及时清理

```bash
# 定期检查 stash 列表
zeta stash list

# 删除不再需要的 stash
zeta stash drop stash@{n}
```

### 7.3 避免长期存储

Stash 是临时存储机制，不应长期保存重要修改：

```bash
# 如果修改很重要，应该创建临时分支
zeta switch -c temp/save-work
zeta add .
zeta commit -m "临时保存"
zeta switch original-branch
```

### 7.4 使用 pop 而非 apply

```bash
# apply 保留 stash 在列表中
zeta stash apply   # 需要手动 drop

# pop 自动删除
zeta stash pop     # 推荐使用
```

## 八、故障排查

### 8.1 Stash 恢复冲突

```
$ zeta stash pop
CONFLICT (content): Merge conflict in file.c
Automatic merge failed; fix conflicts and then commit the result.
```

解决方案：

```bash
# 查看冲突
zeta status

# 编辑冲突文件解决冲突
# ...

# 标记已解决
zeta add file.c

# stash pop 失败时 stash 不会被删除
# 解决冲突后手动删除
zeta stash drop
```

### 8.2 暂存区恢复失败

```
$ zeta stash apply --index
错误：无法恢复暂存区状态
```

解决方案：

```bash
# 不恢复暂存区
zeta stash apply

# 手动 add 需要暂存的文件
zeta add <files>
```

### 8.3 Stash 列表丢失

Stash 存储在 `refs/stash` 引用中：

```bash
# 检查 stash 引用
cat .zeta/refs/stash

# 如果不小心删除了 stash 引用
# 可以在 packed-refs 或 reflog 中查找
```

## 九、内部实现

### 9.1 Stash 引用存储

Stash 使用 `refs/stash` 引用存储最新的 stash entry，每个 entry 的 parent 指向之前的 stash：

```
stash@{0} ← refs/stash
    │
    └── parent → stash@{1}
                    │
                    └── parent → stash@{2}
                                    │
                                    └── ...
```

### 9.2 Stash Entry 结构

```
Stash Entry (提交 B - Worktree State)
├── parent 1: Index Commit (提交 A)
├── parent 2: HEAD Commit
├── tree: 完整的 worktree tree
└── message: stash 描述信息

Index Commit (提交 A)
├── parent: HEAD Commit
├── tree: index tree
└── (无 message)
```

### 9.3 恢复算法

```
1. 读取 stash entry 的两个 parent
2. 计算 HEAD 与 stash worktree commit 的差异
3. 应用差异到当前工作区
4. 如果指定 --index：
   a. 计算 HEAD 与 index commit 的差异
   b. 恢复暂存区状态
```

## 十、相关命令

| 命令 | 说明 |
|-----|------|
| `zeta status` | 查看工作区状态 |
| `zeta add` | 添加修改到暂存区 |
| `zeta reset` | 重置暂存区 |
| `zeta switch` | 切换分支 |
| `zeta commit` | 提交修改 |