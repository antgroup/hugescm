# Zeta 文档中心

欢迎来到 Zeta 文档中心！Zeta 是面向 AI 场景的巨型存储库版本控制系统。

---

## 文档索引

### 设计与架构

| 文档 | 描述 |
|------|------|
| [desgin.md](desgin.md) | HugeSCM 设计哲学 - 核心设计理念、架构概述、与 Git 的差异 |
| [object-format.md](object-format.md) | 对象格式详解 - Blob、Tree、Commit、Fragments 等对象的二进制格式 |
| [pack-format.md](pack-format.md) | Pack 文件格式 - 对象打包机制和索引格式 |
| [protocol.md](protocol.md) | 传输协议规范 - HTTP/SSH 协议、授权、元数据和文件传输 |
| [version-negotiation.md](version-negotiation.md) | 版本协商机制 - 基线管理、检出、拉取、推送流程 |

### 配置参考

| 文档 | 描述 |
|------|------|
| [config.md](config.md) | 配置文件说明 - 支持的配置项和环境变量 |

### 功能使用

| 文档 | 描述 |
|------|------|
| [switch.md](switch.md) | 分支切换 - switch 命令详解，切换分支和提交 |
| [stash.md](stash.md) | 暂存功能 - stash 命令详解，临时保存工作进度 |
| [sparse-checkout.md](sparse-checkout.md) | 稀疏检出 - 按需检出指定目录 |
| [pull-strategy.md](pull-strategy.md) | 拉取策略 - merge、rebase、fast-forward 策略详解 |

### 高级特性

| 文档 | 描述 |
|------|------|
| [cdc.md](cdc.md) | CDC 分片 - Content-Defined Chunking 实现原理和配置 |

---

## 快速开始

### 1. 安装

安装最新版本的 Golang 后，使用以下命令构建：

```sh
# 使用 bali 构建
bali -T linux -A amd64

# 或使用 make
make build
```

### 2. 配置

```shell
# 设置用户信息
zeta config --global user.email 'your@email.com'
zeta config --global user.name 'Your Name'

# 开启 OSS 直连下载（推荐）
zeta config --global core.accelerator direct
```

### 3. 检出存储库

```shell
# 检出存储库
zeta checkout http://zeta.example.io/group/repo my-repo

# 稀疏检出指定目录
zeta checkout http://zeta.example.io/group/repo my-repo -s dir1

# 逐一检出模式（节省磁盘空间）
zeta checkout http://zeta.example.io/group/repo my-repo --one
```

### 4. 基本工作流

```shell
# 查看状态
zeta status

# 添加修改
zeta add <files>

# 提交
zeta commit -m "提交信息"

# 推送
zeta push

# 拉取更新
zeta pull
```

---

## 核心概念

### 数据分离架构

HugeSCM 采用**数据分离原则**：

```
+------------------+     +------------------+
|   元数据数据库    |     |   对象存储/OSS   |
|  (分布式数据库)   |     |  (分布式文件系统) |
+------------------+     +------------------+
        ↑                         ↑
        │                         │
   commit/tree              blob 数据
   fragments/tag            (压缩存储)
```

### Fragments 对象

针对巨型文件，HugeSCM 引入 **Fragments** 对象：

- 将大文件分割为多个 Blob 存储
- 支持 CDC（Content-Defined Chunking）智能分片
- 增量传输，减少带宽消耗

### CDC 分片优势

| 场景 | 传统固定分片 | CDC 分片 |
|------|------------|---------|
| 局部修改 | 所有后续分片改变 | 仅 1-2 个分片改变 |
| 增量同步 | 传输完整文件 | 仅传输变化分片 |
| 去重效果 | 低 | 高 |

启用 CDC：

```toml
[fragment]
threshold = "1GB"      # 文件大小阈值
size = "4GB"           # 目标分片大小
enable_cdc = true      # 启用 CDC 分片
```

---

## 适用场景

### AI 大模型研发

- 存储 checkpoint 文件（数十 GB 到数百 GB）
- 模型版本管理和增量更新
- 多团队协作

### 游戏研发

- 大型二进制资源管理
- 美术资产版本控制

### 数据集存储

- 大规模数据集版本管理
- 数据标注协作

---

## 与 Git 的主要差异

| 特性 | Git | HugeSCM |
|-----|-----|---------|
| 架构模式 | 分布式 | 集中式 |
| 克隆方式 | 全量克隆 | 按需检出 |
| 哈希算法 | SHA-1/SHA-256 | BLAKE3 |
| 大文件支持 | Git LFS | 内置 Fragments |
| 数据存储 | 本地文件系统 | DB + OSS |

### 命令对照

| Git 命令 | HugeSCM 命令 | 说明 |
|---------|-------------|------|
| `git clone` | `zeta checkout` | 检出存储库，非全量克隆 |
| `git fetch` | `zeta pull --fetch` | 仅获取数据 |
| `git pull` | `zeta pull` | 拉取并合并 |
| `git switch` | `zeta switch` | 切换分支 |

---

## 获取帮助

- **命令帮助**：`zeta <command> -h`
- **问题反馈**：提交 Issue 到内部代码仓库
- **技术支持**：联系 Zeta 团队

---

## 文档更新

- 2026-03-18: 补全设计哲学、拉取策略、分支切换、暂存功能文档
- 2026-03-17: 添加 CDC 分片功能文档
- 2025-08-20: 初始文档创建