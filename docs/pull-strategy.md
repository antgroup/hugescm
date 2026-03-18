# HugeSCM Pull 不同策略说明

在 HugeSCM 中，我们引入了与 git pull 相匹配的策略，如下：

1. **merge** - 合并策略（默认）
2. **rebase** - 变基策略
3. **fast-forward only** - 仅快进策略

三种策略有不同的处理流程，适用于不同的协作场景。

## 一、Merge 策略

### 1.1 策略说明

Merge 策略是 HugeSCM 的默认拉取策略。当本地分支相对于远程分支有独立的提交时，会创建一个合并提交（merge commit），将远程分支的变更与本地变更合并。

### 1.2 工作流程

```
远程分支: A --- B --- C --- D
                    \
本地分支:            E --- F

执行 pull --merge 后:

远程分支: A --- B --- C --- D
                    \         \
本地分支:            E --- F --- M (合并提交)
```

### 1.3 使用场景

- 团队协作开发，多人同时在不同分支工作
- 需要保留完整的分支历史
- 需要清晰看到何时进行了合并

### 1.4 冲突处理

当本地修改与远程修改冲突时：

1. HugeSCM 会标记冲突文件
2. 用户需要手动解决冲突
3. 解决冲突后执行 `zeta add` 和 `zeta commit`
4. 完成合并后推送变更

冲突标记格式（默认使用 `merge` 风格）：

```
<<<<<<< HEAD
本地修改内容
=======
远程修改内容
>>>>>>> remote
```

### 1.5 命令示例

```bash
# 默认使用 merge 策略
zeta pull

# 显式指定 merge 策略
zeta pull --merge

# 指定冲突样式
zeta pull --merge --conflict-style=diff3
```

---

## 二、Rebase 策略

### 2.1 策略说明

Rebase 策略将本地提交"重新应用"到远程分支的最新提交之上，保持线性历史，避免产生合并提交。

### 2.2 工作流程

```
远程分支: A --- B --- C --- D
                    \
本地分支:            E --- F

执行 pull --rebase 后:

远程分支: A --- B --- C --- D --- E' --- F'
                              ↑
                         重新应用的提交
```

### 2.3 使用场景

- 保持提交历史的线性，便于理解
- 避免不必要的合并提交
- 代码审查时历史更清晰

### 2.4 注意事项

- **不要对已推送的提交执行 rebase**：这会改变提交历史，影响其他协作者
- Rebase 会重写提交哈希，原始提交将无法直接访问
- 冲突需要逐个提交解决

### 2.5 冲突处理

Rebase 过程中遇到冲突：

1. HugeSCM 会暂停 rebase 过程
2. 用户解决当前提交的冲突
3. 执行 `zeta add` 标记冲突已解决
4. 执行 `zeta rebase --continue` 继续 rebase
5. 或执行 `zeta rebase --abort` 放弃 rebase

### 2.6 命令示例

```bash
# 使用 rebase 策略拉取
zeta pull --rebase

# 自动暂存本地修改后 rebase
zeta pull --rebase --autostash
```

---

## 三、Fast-forward Only 策略

### 3.1 策略说明

Fast-forward Only 策略仅在可以进行快进合并时执行合并。如果本地分支有独立提交（即无法快进），则拒绝合并。

### 3.2 工作流程

**可以快进的情况：**

```
远程分支: A --- B --- C --- D
              \
本地分支:      C

执行 pull --ff-only 后:

本地分支: A --- B --- C --- D  (快进到 D)
```

**无法快进的情况：**

```
远程分支: A --- B --- C --- D
                    \
本地分支:            E

执行 pull --ff-only 后:
报错：无法快进合并，操作被拒绝
```

### 3.3 使用场景

- 需要严格保持线性历史
- 禁止在本地进行独立开发
- CI/CD 环境中确保干净的合并

### 3.4 与 --ff 的区别

| 选项 | 可快进时 | 不可快进时 |
|-----|---------|-----------|
| `--ff` | 执行快进 | 执行合并（创建合并提交） |
| `--ff-only` | 执行快进 | 拒绝合并，报错退出 |

### 3.5 命令示例

```bash
# 仅允许快进合并
zeta pull --ff-only

# 组合使用
zeta pull --ff-only --autostash
```

---

## 四、策略对比

| 特性 | Merge | Rebase | Fast-forward Only |
|-----|-------|--------|------------------|
| 历史类型 | 非线性 | 线性 | 线性 |
| 合并提交 | 产生 | 不产生 | 不产生 |
| 冲突处理 | 一次解决 | 逐提交解决 | 不适用 |
| 适用场景 | 团队协作 | 个人分支 | 严格流程 |
| 历史可读性 | 完整但复杂 | 清晰 | 最清晰 |
| 安全性 | 高 | 中（可能改写历史） | 高 |

---

## 五、配置

### 5.1 设置默认策略

可以通过配置设置默认的拉取策略：

```bash
# 设置默认使用 rebase 策略
zeta config pull.rebase true

# 设置默认仅使用快进合并
zeta config pull.ff only
```

### 5.2 Autostash 配置

自动暂存本地修改，在 pull 完成后恢复：

```bash
# 启用 autostash
zeta config pull.autostash true
```

### 5.3 冲突样式配置

配置合并时的冲突标记样式：

```bash
# 可选值: merge, diff3, zdiff3
zeta config merge.conflictStyle diff3
```

**diff3 样式示例：**

```
<<<<<<< HEAD
本地修改
||||||| 基准版本
原始内容
=======
远程修改
>>>>>>> remote
```

---

## 六、最佳实践

### 6.1 团队协作推荐

```
1. 在共享分支上使用 merge 或 ff-only
2. 个人特性分支使用 rebase 保持整洁
3. 已推送的提交不要 rebase
```

### 6.2 工作流建议

**Git Flow 风格：**

- 主分支：使用 `--ff-only`
- 开发分支：使用 `--merge`
- 特性分支：rebase 到开发分支

**GitHub Flow 风格：**

- 主分支：使用 `--ff-only`
- 特性分支：通过 PR 合并

### 6.3 常见问题

**Q: pull 失败提示 "cannot fast-forward"**

A: 本地有未推送的提交，且远程分支有新提交。选择：
- 使用 `--merge` 创建合并提交
- 使用 `--rebase` 变基本地提交

**Q: rebase 过程中想放弃怎么办？**

A: 执行 `zeta rebase --abort` 恢复到 rebase 前的状态。

**Q: 如何查看当前分支与远程分支的差异？**

A: 执行 `zeta log HEAD..@{u}` 查看远程领先的提交。

---

## 七、与 Git 的兼容性

HugeSCM 的 pull 策略设计与 Git 保持一致，熟悉 Git 的用户可以无缝切换：

| Git 命令 | HugeSCM 命令 |
|---------|-------------|
| `git pull` | `zeta pull` |
| `git pull --rebase` | `zeta pull --rebase` |
| `git pull --ff-only` | `zeta pull --ff-only` |
| `git pull --no-rebase` | `zeta pull --merge` |

主要差异在于 HugeSCM 是集中式架构，pull 操作从远程获取指定分支的数据，而非全量获取所有远程分支。