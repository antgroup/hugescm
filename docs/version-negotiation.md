# 版本协商备忘录

本文档描述 HugeSCM 的版本协商机制，包括分支基线、检出、拉取、合并和推送等核心操作的流程。

## 一、分支基线

### 1.1 基线概念

在 HugeSCM 客户端，我们存在一个**分支基线（Baseline）**的概念。这个基线标记了存储库从远程存储库的某个提交开始向前发展，计算对象变更时会从基线开始计算。对于多个分支，我们会保留多个基线。

我们在 Fetch/Push 这些阶段严格依赖基线以实现版本协商。

### 1.2 与 Git Shallow 的对比

这和 Git 类似，Git 浅表克隆+稀疏检出时，会在存储库中保留一个 `shallow` 文件。但该文件是全局的，因此无法对多个分支实现 shallow 控制。在拉取其他分支时，往往也需要依赖此 shallow 文件。除非用户更改，否则 shallow 是不改变的，这样的结果是 Git shallow 克隆的仓库体积还是会随着时间膨胀。

HugeSCM 的改进：
- 每个分支独立维护基线
- 支持多分支独立的浅表控制
- 基线可以动态调整

### 1.3 基线重置

在 Fetch/Push 后，远程分支发生改变后，客户端可以修改分支基线到最新的 commit：

```bash
# 拉取时自动更新基线
zeta pull

# 获取完整历史
zeta fetch --unshallow

# 推送后更新基线
zeta push
```

### 1.4 基线存储

基线信息存储在 `.zeta/refs/` 目录下：

```
.zeta/
├── refs/
│   ├── branches/
│   │   └── mainline      # 包含 hash 和 baseline
│   └── tags/
│       └── v1.0.0
```

## 二、检出（Checkout）

### 2.1 检出流程

检出 ==> 拉取 + 重置

在 HugeSCM 中，我们将远程存储库创建到本地的浅表副本，该操作称之为检出（checkout），别名 `co`。

其步骤如下：

1. **初始化存储库本地目录**
   - 创建工作目录
   - 创建 `.zeta` 目录结构
   - 生成初始配置文件

2. **获取引用信息**
   - 使用引用发现协议获取分支/标签信息
   - 对于检出特定 commit 的操作，忽略引用发现获得的 commit/peeled commit

3. **获取元数据**
   - 使用获取的 commit 或特定 commit 获取元数据
   - 可指定深度（deepen）和目录（sparse）

4. **拉取对象**
   - 批量下载 blobs（小文件）
   - 下载大的 blobs（如有需要）
   - 对象清点基于第三步获得的对象

5. **重置索引，检出文件**
   - 更新索引
   - 检出文件到工作区
   - 设置分支基线

### 2.2 检出命令

```bash
# 基本检出
zeta checkout http://zeta.example.io/group/repo myrepo

# 检出特定分支
zeta checkout http://zeta.example.io/group/repo myrepo -b feature

# 检出特定标签
zeta checkout http://zeta.example.io/group/repo myrepo -t v1.0.0

# 检出特定提交
zeta checkout http://zeta.example.io/group/repo myrepo --commit=abc123...

# 稀疏检出
zeta checkout http://zeta.example.io/group/repo myrepo -s dir1 -s dir2

# 浅表检出（只获取最近 N 个提交）
zeta checkout http://zeta.example.io/group/repo myrepo --depth=1
```

### 2.3 检出选项

| 选项 | 说明 |
|-----|------|
| `-b, --branch=<name>` | 检出并创建本地分支 |
| `-t, --tag=<name>` | 检出特定标签 |
| `--commit=<commit>` | 检出特定提交 |
| `-s, --sparse=<dir>` | 稀疏检出目录 |
| `--depth=<n>` | 浅表检出深度 |
| `-L, --limit=<size>` | 限制检出文件大小 |
| `--one` | 逐一检出模式 |

## 三、拉取（Pull）

### 3.1 拉取流程

在 HugeSCM 中，从服务端拉取数据的步骤：

1. **获得远程引用信息**
   - 使用引用发现协议
   - 获取远程分支最新提交

2. **下载元数据**
   - 基于 baseline 参数
   - 获取 commit、tree、fragments 等元数据

3. **批量下载 blobs**
   - 小文件批量下载
   - 支持并发下载

4. **下载大的 blobs**
   - 使用签名 URL 下载
   - 支持断点续传

5. **记录引用信息到本地**
   - 更新本地分支引用
   - 更新基线信息

### 3.2 拉取命令

```bash
# 基本拉取（合并模式）
zeta pull

# 使用 rebase 策略
zeta pull --rebase

# 仅快进合并
zeta pull --ff-only

# 获取完整历史
zeta pull --unshallow

# 限制文件大小
zeta pull -L 100MB
```

### 3.3 拉取选项

| 选项 | 说明 |
|-----|------|
| `--[no-]ff` | 允许快进（默认开启） |
| `--ff-only` | 仅允许快进合并 |
| `--rebase` | 使用 rebase 策略 |
| `--squash` | 创建单个提交而非合并 |
| `--unshallow` | 获取完整历史 |
| `--one` | 逐一检出大文件 |
| `-L, --limit=<size>` | 限制文件大小 |

### 3.4 获取（Fetch）

如果只想获取数据而不合并：

```bash
# 获取远程数据
zeta fetch

# 获取特定引用
zeta fetch mainline

# 获取完整历史
zeta fetch --unshallow

# 仅获取标签
zeta fetch --tag
```

Fetch 选项：

| 选项 | 说明 |
|-----|------|
| `--unshallow` | 获取完整历史 |
| `-t, --tag` | 下载标签而非分支 |
| `-L, --limit=<size>` | 限制文件大小 |
| `-f, --force` | 覆盖引用检查 |

## 四、合并（Merge）

### 4.1 合并流程

当本地分支与远程分支有分叉时，需要进行合并：

1. **检测分叉**
   - 比较本地和远程的提交历史
   - 确定共同祖先

2. **三路合并**
   - 以共同祖先为基准
   - 合并本地和远程的变更

3. **冲突处理**
   - 自动合并可解决的冲突
   - 标记需要手动解决的冲突

4. **创建合并提交**
   - 记录合并结果
   - 保持历史完整

### 4.2 合并命令

```bash
# 合并指定分支
zeta merge feature

# 合并并编辑提交信息
zeta merge feature -m "Merge feature"

# 快进合并（默认）
zeta merge feature --ff

# 仅快进合并
zeta merge feature --ff-only

# 强制创建合并提交
zeta merge feature --no-ff

# 创建 squash 提交
zeta merge feature --squash

# 中止合并
zeta merge --abort

# 继续合并（解决冲突后）
zeta merge --continue
```

### 4.3 冲突解决

当合并产生冲突时：

```bash
# 查看冲突文件
zeta status

# 编辑冲突文件
# 解决冲突标记：
# <<<<<<< HEAD
# 本地修改
# =======
# 远程修改
# >>>>>>> feature

# 标记冲突已解决
zeta add <conflicted-file>

# 继续合并
zeta merge --continue
```

### 4.4 冲突样式

可通过配置设置冲突标记样式：

```bash
# merge 样式（默认）
zeta config merge.conflictStyle merge

# diff3 样式（显示基准版本）
zeta config merge.conflictStyle diff3

# zdiff3 样式（压缩的 diff3）
zeta config merge.conflictStyle zdiff3
```

### 4.5 合并选项

| 选项 | 说明 |
|-----|------|
| `--[no-]ff` | 允许快进（默认开启） |
| `--ff-only` | 仅快进合并 |
| `--squash` | 创建单个提交 |
| `--allow-unrelated-histories` | 允许合并不相关历史 |
| `-m, --message=<msg>` | 合并提交信息 |
| `--abort` | 中止合并 |
| `--continue` | 继续合并 |

## 五、推送（Push）

### 5.1 推送流程

将本地变更推送到远程存储库：

1. **对象上传**
   - 上传新的 blob 对象
   - 上传新的元数据对象

2. **引用更新**
   - 更新远程分支引用
   - 验证权限

3. **基线更新**
   - 更新本地基线信息

### 5.2 推送前检查

```bash
# 查看待推送的提交
zeta log origin/mainline..HEAD

# 查看待推送的变更
zeta diff origin/mainline --stat
```

### 5.3 推送命令

```bash
# 推送当前分支
zeta push

# 推送标签
zeta push --tag

# 强制推送
zeta push --force

# 推送并传递选项
zeta push -o option=value
```

### 5.4 推送选项

| 选项 | 说明 |
|-----|------|
| `-t, --tag` | 推送标签 |
| `-f, --force` | 强制推送 |
| `-o, --push-option=<opt>` | 传输选项 |

### 5.5 推送保护

服务端会进行以下检查：

- 分支是否存在
- 是否为保护分支
- 用户是否有写权限
- 是否为快进更新（非强制推送）

## 六、版本协商协议

### 6.1 协议版本

当前协议版本为 `Z1`：

- HTTP 请求设置头：`Zeta-Protocol: Z1`
- SSH 请求设置环境变量：`ZETA_PROTOCOL=Z1`

### 6.2 基线协商

在 Fetch/Push 时，客户端会发送基线信息：

```
客户端请求：
  I have: <local-baseline-commit>
  I want: <remote-head-commit>

服务端响应：
  需要发送的对象列表
  或 增量元数据
```

### 6.3 增量传输

基于基线的增量传输：

- **第一次检出**：从空状态获取 commit 及其所有对象
- **后续拉取**：基于 baseline 获取增量对象
- **推送**：发送 baseline 到本地 HEAD 之间的增量对象

## 七、最佳实践

### 7.1 定期拉取

```bash
# 建议定期拉取更新
zeta pull --rebase
```

### 7.2 推送前检查

```bash
# 检查待推送内容
zeta log origin/mainline..HEAD --oneline
zeta diff origin/mainline --stat
```

### 7.3 解决冲突

```bash
# 拉取时产生冲突
zeta pull
# 解决冲突...
zeta add .
zeta commit
zeta push
```

### 7.4 保持基线更新

```bash
# 定期获取更多历史，减少增量传输
zeta fetch --unshallow
```

## 八、相关文档

| 文档 | 说明 |
|------|------|
| [protocol.md](protocol.md) | 传输协议规范 |
| [pull-strategy.md](pull-strategy.md) | 拉取策略详解 |
| [sparse-checkout.md](sparse-checkout.md) | 稀疏检出 |
| [switch.md](switch.md) | 分支切换 |