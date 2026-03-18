# HugeSCM 配置文件说明

本文档详细说明 HugeSCM 支持的配置项和环境变量。

## 一、配置层级

HugeSCM 的配置系统支持三个层级（优先级从低到高）：

| 层级 | 位置 | 说明 |
|------|------|------|
| System | `/etc/zeta.toml` | 系统级配置，所有用户共享 |
| Global | `~/.zeta.toml` | 用户级配置，当前用户所有仓库共享 |
| Local | `.zeta/zeta.toml` | 仓库级配置，仅当前仓库有效 |

**优先级规则**：高优先级配置覆盖低优先级配置。

## 二、配置命令

### 2.1 查看配置

```bash
# 查看所有配置
zeta config --list

# 查看特定配置项
zeta config user.name
zeta config core.accelerator

# 查看特定层级的配置
zeta config --global --list
zeta config --local --list
```

### 2.2 设置配置

```bash
# 设置全局配置
zeta config --global user.name "Your Name"
zeta config --global user.email "your@email.com"

# 设置仓库级配置
zeta config core.accelerator direct

# 添加配置项（多值）
zeta config --add core.sparse "src/core"
```

### 2.3 删除配置

```bash
# 删除配置项
zeta config --unset core.accelerator

# 删除所有匹配的配置
zeta config --unset-all core.sparse
```

### 2.4 重命名配置

```bash
# 重命名配置节
zeta config --rename-section old.name new.name
```

## 三、配置文件格式

配置文件采用 TOML 格式：

```toml
# 用户信息
[user]
name = "Your Name"
email = "your@email.com"

# 核心配置
[core]
remote = "https://zeta.example.io/group/repo"
accelerator = "direct"
concurrenttransfers = 10

# 分片配置
[fragment]
threshold = "1GB"
size = "4GB"
enable_cdc = true

# HTTP 配置
[http]
sslVerify = true
```

## 四、核心配置项

### 4.1 用户信息

| 配置项 | 环境变量 | 说明 | 示例 |
|--------|----------|------|------|
| `user.name` | `ZETA_AUTHOR_NAME` | 作者名 | `"John Doe"` |
| | `ZETA_COMMITTER_NAME` | 提交者名 | `"John Doe"` |
| `user.email` | `ZETA_AUTHOR_EMAIL` | 作者邮箱 | `"john@example.com"` |
| | `ZETA_COMMITTER_EMAIL` | 提交者邮箱 | `"john@example.com"` |
| | `ZETA_AUTHOR_DATE` | 作者签名时间 | `"2024-01-01T00:00:00"` |
| | `ZETA_COMMITTER_DATE` | 提交时间 | `"2024-01-01T00:00:00"` |

### 4.2 存储库配置

| 配置项 | 环境变量 | 说明 | 默认值 |
|--------|----------|------|--------|
| `core.remote` | | 远程存储库地址 | - |
| `core.sparse` | | 稀疏检出目录列表 | `[]` |
| `core.sharingRoot` | `ZETA_CORE_SHARING_ROOT` | Blob 共享存储根目录 | - |
| `core.optimizeStrategy` | `ZETA_CORE_OPTIMIZE_STRATEGY` | 空间管理策略 | - |

### 4.3 传输配置

| 配置项 | 环境变量 | 说明 | 默认值 |
|--------|----------|------|--------|
| `core.accelerator` | `ZETA_CORE_ACCELERATOR` | 下载加速器 | - |
| `core.concurrenttransfers` | `ZETA_CORE_CONCURRENT_TRANSFERS` | 并发下载数（1-50） | - |
| | `ZETA_CORE_PROMISOR` | 按需下载标志 | `true` |

### 4.4 编辑器配置

| 配置项 | 环境变量 | 说明 | 备注 |
|--------|----------|------|------|
| `core.editor` | `ZETA_EDITOR` | 提交信息编辑器 | 兼容 `GIT_EDITOR`、`EDITOR` |

## 五、HTTP 配置

### 5.1 SSL 配置

| 配置项 | 环境变量 | 说明 | 默认值 |
|--------|----------|------|--------|
| `http.sslVerify` | `ZETA_SSL_NO_VERIFY` | SSL 验证 | `true` |

注意：`ZETA_SSL_NO_VERIFY=true` 与 `http.sslVerify=false` 效果相同。

### 5.2 HTTP 头配置

| 配置项 | 说明 |
|--------|------|
| `http.extraHeader` | 设置 HTTP 附加头 |

```bash
# 设置附加 HTTP 头
zeta config http.extraHeader "X-Custom-Header: value"

# 设置 Authorization 跳过权限预验证
zeta config http.extraHeader "Authorization: Bearer token"
```

## 六、传输层配置

| 配置项 | 环境变量 | 说明 | 默认值 |
|--------|----------|------|--------|
| `transport.maxEntries` | `ZETA_TRANSPORT_MAX_ENTRIES` | Batch 下载对象数量限制 | - |
| `transport.largeSize` | `ZETA_TRANSPORT_LARGE_SIZE` | 大文件大小阈值 | `5M` |
| `transport.externalProxy` | `ZETA_TRANSPORT_EXTERNAL_PROXY` | Direct 下载外部代理 | - |

## 七、Diff 和 Merge 配置

### 7.1 Diff 配置

| 配置项 | 说明 | 可选值 |
|--------|------|--------|
| `diff.algorithm` | Diff 算法 | `histogram`、`onp`、`myers`、`patience`、`minimal` |

```bash
# 设置 diff 算法
zeta config diff.algorithm histogram
```

### 7.2 Merge 配置

| 配置项 | 说明 | 可选值 |
|--------|------|--------|
| `merge.conflictStyle` | 冲突标记样式 | `merge`、`diff3`、`zdiff3` |

| 环境变量 | 说明 |
|----------|------|
| `ZETA_MERGE_TEXT_DRIVER` | 文本合并工具，可设置为 `git` 使用 git merge-file |

```bash
# 设置冲突样式
zeta config merge.conflictStyle diff3

# 使用 git 作为合并工具
export ZETA_MERGE_TEXT_DRIVER=git
```

## 八、终端配置

| 环境变量 | 说明 |
|----------|------|
| `ZETA_PAGER` / `PAGER` | 终端分页工具，默认搜索 `less` |
| `ZETA_TERMINAL_PROMPT` | 设为 `false` 禁用终端交互 |

```bash
# 禁用分页
export PAGER=""

# 禁用终端交互
export ZETA_TERMINAL_PROMPT=false
```

## 九、分片配置

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `fragment.threshold` | Size | `1GB` | 文件大小阈值，小于此值不分片 |
| `fragment.size` | Size | `1GB` | 目标分片大小（固定分片） |
| `fragment.enable_cdc` | Boolean | `false` | 启用 CDC 分片 |

### 9.1 Size 格式

支持以下单位：

- `KB`、`MB`、`GB`（1000 进制）
- `KiB`、`MiB`、`GiB`（1024 进制）

```toml
[fragment]
threshold = "512MiB"
size = "1GB"
enable_cdc = true
```

### 9.2 配置层级合并

Boolean 类型支持配置层级合并：

```go
// 高优先级配置覆盖低优先级配置
func (b *Boolean) Merge(other *Boolean) {
    if other.val != BOOLEAN_UNSET {
        b.val = other.val
    }
}
```

## 十、下载加速器配置

| 加速器 | 说明 | 适用场景 |
|--------|------|----------|
| `direct` | 直接从 OSS 签名 URL 下载 | AI 场景，高速内网 |
| `dragonfly` | 使用 Dragonfly P2P 加速 | 大规模分布式环境 |
| `aria2` | 使用 aria2c 多线程下载 | 个人开发环境 |

```bash
# 设置加速器
zeta config --global core.accelerator direct

# 设置 Dragonfly 路径
export ZETA_EXTENSION_DRAGONFLY_GET=/path/to/dfget

# 设置 aria2 路径
export ZETA_EXTENSION_ARIA2C=/path/to/aria2c
```

## 十一、完整配置示例

### 11.1 全局配置示例 (`~/.zeta.toml`)

```toml
[user]
name = "John Doe"
email = "john@example.com"

[core]
accelerator = "direct"
concurrenttransfers = 10
editor = "vim"

[http]
sslVerify = true

[diff]
algorithm = "histogram"

[merge]
conflictStyle = "diff3"

[fragment]
enable_cdc = true
threshold = "1GB"
size = "1GB"
```

### 11.2 仓库配置示例 (`.zeta/zeta.toml`)

```toml
[core]
remote = "https://zeta.example.io/group/repo"
sparse = ["src/core", "src/utils"]
compression-algo = "zstd"
```

## 十二、配置速查表

| 配置 | 环境变量 | 说明 |
|:-----|:---------|:-----|
| `core.sharingRoot` | `ZETA_CORE_SHARING_ROOT` | Blob 共享存储根目录 |
| `core.sparse` | | 稀疏检出目录配置 |
| `core.remote` | | 远程存储库地址 |
| `user.name` | `ZETA_AUTHOR_NAME` / `ZETA_COMMITTER_NAME` | 用户名 |
| `user.email` | `ZETA_AUTHOR_EMAIL` / `ZETA_COMMITTER_EMAIL` | 用户邮箱 |
| | `ZETA_AUTHOR_DATE` / `ZETA_COMMITTER_DATE` | 签名时间 |
| `core.accelerator` | `ZETA_CORE_ACCELERATOR` | 下载加速器 |
| `core.optimizeStrategy` | `ZETA_CORE_OPTIMIZE_STRATEGY` | 空间管理策略 |
| `core.concurrenttransfers` | `ZETA_CORE_CONCURRENT_TRANSFERS` | 并发下载数 |
| | `ZETA_CORE_PROMISOR` | 按需下载标志 |
| `core.editor` | `ZETA_EDITOR` / `GIT_EDITOR` / `EDITOR` | 编辑器 |
| | `ZETA_MERGE_TEXT_DRIVER` | 文本合并工具 |
| | `ZETA_SSL_NO_VERIFY` | 禁用 SSL 验证 |
| `http.sslVerify` | | SSL 验证（与上相反） |
| `http.extraHeader` | | HTTP 附加头 |
| `transport.maxEntries` | `ZETA_TRANSPORT_MAX_ENTRIES` | Batch 下载限制 |
| `transport.largeSize` | `ZETA_TRANSPORT_LARGE_SIZE` | 大文件阈值 |
| `transport.externalProxy` | `ZETA_TRANSPORT_EXTERNAL_PROXY` | 外部代理 |
| `diff.algorithm` | | Diff 算法 |
| `merge.conflictStyle` | | 冲突样式 |
| | `ZETA_PAGER` / `PAGER` | 分页工具 |
| | `ZETA_TERMINAL_PROMPT` | 终端交互 |

## 十三、相关文档

| 文档 | 说明 |
|------|------|
| [desgin.md](desgin.md) | 设计哲学 |
| [sparse-checkout.md](sparse-checkout.md) | 稀疏检出 |
| [cdc.md](cdc.md) | CDC 分片配置 |