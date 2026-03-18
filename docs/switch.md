# Switch - 切换分支和提交

`zeta switch` 命令用于切换工作区到不同的分支或提交。与 Git 的 `git switch` / `git checkout` 类似，但针对 HugeSCM 的集中式架构进行了优化。

## 一、基本用法

### 1.1 切换分支

```bash
# 切换到已存在的本地分支
zeta switch feature-branch

# 切换到远程分支（自动创建本地跟踪分支）
zeta switch origin/feature-branch
```

### 1.2 创建并切换分支

```bash
# 从当前分支创建新分支并切换
zeta switch -c new-feature

# 从指定提交创建新分支
zeta switch -c new-feature abc123

# 从远程分支创建本地分支
zeta switch -c new-feature origin/mainline
```

### 1.3 切换到特定提交

```bash
# 切换到特定提交（分离 HEAD 状态）
zeta switch abc123def456...

# 使用短哈希
zeta switch abc123
```

### 1.4 切换到标签

```bash
# 切换到标签（分离 HEAD 状态）
zeta switch v1.0.0
```

## 二、命令选项

| 选项 | 说明 |
|-----|------|
| `-c, --create <name>` | 创建新分支并切换 |
| `-C, --force-create <name>` | 强制创建分支（覆盖已存在的分支） |
| `-d, --detach` | 切换到提交时强制进入分离 HEAD 状态 |
| `--discard-changes` | 丢弃本地未提交的修改 |
| `-f, --force` | 强制切换（等同于 --discard-changes） |
| `-m, --merge` | 切换时合并本地修改到目标分支（默认开启） |
| `--no-merge` | 禁用合并模式 |
| `--orphan` | 创建孤儿分支 |
| `--remote` | 当分支不存在时尝试从远程获取 |
| `-L, --limit <size>` | 限制检出文件大小 |
| `--quiet` | 静默模式 |

## 三、切换行为详解

### 3.1 正常切换

当工作区干净或本地修改与目标分支无冲突时：

```
当前分支: mainline (有未提交修改)
目标分支: feature (与修改无冲突)

执行: zeta switch feature
结果: 成功切换，本地修改保留
```

### 3.2 有冲突的切换

当本地修改与目标分支有冲突时：

```bash
# 方式一：强制切换，丢弃本地修改
zeta switch --force feature

# 方式二：合并本地修改到目标分支
zeta switch --merge feature

# 方式三：暂存修改后切换
zeta stash
zeta switch feature
zeta stash pop
```

### 3.3 分离 HEAD 状态

切换到特定提交或标签时，进入分离 HEAD 状态：

```
$ zeta switch abc123
注意：您正处于分离 HEAD 状态。
您可以查看、进行实验性修改并提交，这些更改不会影响任何分支。
如果您想以当前状态创建新分支，请使用：
  zeta switch -c <新分支名>
```

在分离 HEAD 状态下的提交不会被任何分支引用，切换到其他分支后可能丢失。建议：

```bash
# 在分离 HEAD 状态下创建新分支保存工作
zeta switch -c my-work
```

## 四、分支创建

### 4.1 从当前分支创建

```bash
# 从当前 HEAD 创建新分支
zeta switch -c feature-123

# 等价于
zeta branch feature-123
zeta switch feature-123
```

### 4.2 从指定起点创建

```bash
# 从指定提交创建
zeta switch -c feature-123 abc123

# 从远程分支创建
zeta switch -c feature-123 origin/mainline

# 从标签创建
zeta switch -c v1.0-hotfix v1.0.0
```

### 4.3 强制创建/覆盖

```bash
# 覆盖已存在的分支
zeta switch -C existing-branch origin/mainline
```

## 五、与 Git 的差异

### 5.1 远程分支处理

**Git：**

```bash
git switch origin/feature
# 进入分离 HEAD 状态
```

**HugeSCM：**

```bash
zeta switch origin/feature
# 自动创建本地跟踪分支 feature
```

HugeSCM 由于是集中式架构，切换到远程分支会自动创建本地分支。

### 5.2 数据获取

**Git：**

需要先 `git fetch` 获取远程数据才能切换到远程分支。

**HugeSCM：**

切换时会自动从服务端获取所需的元数据和对象，无需手动 fetch。

### 5.3 网络依赖

HugeSCM 的 switch 操作需要网络连接（除非目标分支数据已完整缓存）。

## 六、常见场景

### 6.1 开始新功能开发

```bash
# 从主分支创建新功能分支
zeta switch mainline
zeta pull
zeta switch -c feature/new-feature
```

### 6.2 切换到同事的分支

```bash
# 直接切换，自动获取数据
zeta switch origin/colleague-feature
```

### 6.3 回退到历史版本

```bash
# 切换到指定提交查看历史状态
zeta switch abc123

# 创建分支保存修改
zeta switch -c hotfix-branch
```

### 6.4 放弃当前修改

```bash
# 丢弃所有未提交的修改
zeta switch --force HEAD
```

## 七、最佳实践

### 7.1 切换前检查状态

```bash
# 查看当前状态
zeta status

# 如果有未提交的修改
zeta stash        # 暂存修改
zeta switch ...   # 切换分支
zeta stash pop    # 恢复修改
```

### 7.2 分支命名规范

```bash
# 推荐使用规范的分支前缀
zeta switch -c feature/user-authentication
zeta switch -c bugfix/login-error
zeta switch -c release/v1.0.0
zeta switch -c hotfix/security-patch
```

### 7.3 避免长时间处于分离 HEAD 状态

```bash
# 不推荐：在分离 HEAD 状态下工作
zeta switch abc123
# ... 进行修改和提交（可能丢失）

# 推荐：立即创建分支
zeta switch abc123
zeta switch -c my-work
```

## 八、故障排查

### 8.1 切换失败：本地修改冲突

```
错误：本地修改与目标分支冲突，无法切换
```

解决方案：

```bash
# 方案一：暂存修改
zeta stash
zeta switch <target>
zeta stash pop

# 方案二：丢弃修改
zeta switch --force <target>

# 方案三：尝试合并
zeta switch --merge <target>
```

### 8.2 切换失败：分支不存在

```
错误：分支 'feature' 不存在
```

解决方案：

```bash
# 检查远程分支
zeta branch -r

# 如果远程存在，使用完整名称
zeta switch origin/feature
```

### 8.3 网络错误

```
错误：无法连接到远程服务器
```

解决方案：

```bash
# 检查网络连接
ping zeta.example.io

# 检查远程配置
zeta config core.remote

# 如果数据已缓存，可尝试离线模式
ZETA_OFFLINE=1 zeta switch <local-branch>
```

## 九、相关命令

| 命令 | 说明 |
|-----|------|
| `zeta branch` | 列出、创建、删除分支 |
| `zeta checkout` | switch 的别名 |
| `zeta stash` | 暂存工作区修改 |
| `zeta status` | 查看工作区状态 |
| `zeta log` | 查看提交历史 |