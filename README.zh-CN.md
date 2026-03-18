# HugeSCM - 基于云的下一代版本控制系统

[![license badge](https://img.shields.io/github/license/antgroup/hugescm.svg)](LICENSE)
[![Master Branch Status](https://github.com/antgroup/hugescm/workflows/CI/badge.svg)](https://github.com/antgroup/hugescm/actions)
[![Latest Release Downloads](https://img.shields.io/github/downloads/antgroup/hugescm/latest/total.svg)](https://github.com/antgroup/hugescm/releases/latest)
[![Total Downloads](https://img.shields.io/github/downloads/antgroup/hugescm/total.svg)](https://github.com/antgroup/hugescm/releases)
[![Version](https://img.shields.io/github/v/release/antgroup/hugescm)](https://github.com/antgroup/hugescm/releases/latest)

[English](./README.md)

## 概述

HugeSCM（代号 zeta）是云原生版本控制系统，专为大规模存储库设计。通过元数据与文件数据分离，突破了 Git/SVN 等传统版本控制系统在存储和传输上的限制。适用于 AI 大模型研发、游戏研发、单一大库等场景。

核心特性：
+ **数据分离**：元数据存储于分布式数据库，文件内容存储于对象存储
+ **高效传输**：优化传输协议，降低带宽和时间成本
+ **分片对象**：高效处理大文件（AI 模型、二进制依赖等）

吸取 Git 经验，摆脱历史包袱。

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

## 文档

### 设计与架构

| 文档 | 描述 |
|------|------|
| [design.md](./docs/design.md) | 设计哲学 - 核心设计理念、架构概述、与 Git 的差异 |
| [object-format.md](./docs/object-format.md) | 对象格式详解 - Blob、Tree、Commit、Fragments 等对象的二进制格式 |
| [pack-format.md](./docs/pack-format.md) | Pack 文件格式 - 对象打包机制和索引格式 |
| [protocol.md](./docs/protocol.md) | 传输协议规范 - HTTP/SSH 协议、授权、元数据和文件传输 |
| [version-negotiation.md](./docs/version-negotiation.md) | 版本协商机制 - 基线管理、检出、拉取、推送流程 |

### 配置参考

| 文档 | 描述 |
|------|------|
| [config.md](./docs/config.md) | 配置文件说明 - 支持的配置项和环境变量 |

### 功能使用

| 文档 | 描述 |
|------|------|
| [switch.md](./docs/switch.md) | 分支切换 - switch 命令详解，切换分支和提交 |
| [stash.md](./docs/stash.md) | 暂存功能 - stash 命令详解，临时保存工作进度 |
| [sparse-checkout.md](./docs/sparse-checkout.md) | 稀疏检出 - 按需检出指定目录 |
| [pull-strategy.md](./docs/pull-strategy.md) | 拉取策略 - merge、rebase、fast-forward 策略详解 |

### 高级特性

| 文档 | 描述 |
|------|------|
| [cdc.md](./docs/cdc.md) | CDC 分片 - Content-Defined Chunking 实现原理和配置 |
| [hot.md](./docs/hot.md) | hot 命令 - Git 存储库维护工具，清理大文件、删除敏感数据、迁移对象格式 |

## 构建

开发者安装好最新版本的 Golang 后，可以使用 [bali](https://github.com/balibuild/bali)（构建打包工具）构建 HugeSCM 客户端。

```sh
bali -T windows
# create rpm,deb,tar,sh pack
bali -T linux -A amd64 --pack='rpm,deb,tar,sh'
```

bali 构建工具可以制作 `zip`, `deb`, `tar`, `rpm`, `sh (STGZ)` 压缩/安装包。

### Windows 安装包

我们提供了 Inno Setup 脚本，可以使用 Docker + wine 在非 Windows 环境下生成安装包：

```shell
docker run --rm -i -v "$TOPLEVEL:/work" amake/innosetup xxxxx.iss
```

运行前请先构建 Windows 二进制：`bali --target=windows --arch=amd64`。

> 注意：在搭载 Apple Silicon 芯片的 macOS 上，可以使用 OrbStack 开启 Rosetta 运行该镜像。

## 使用

用户可以运行 `zeta -h` 查看 zeta 所有命令，并运行 `zeta ${command} -h` 查看命令详细帮助，我们尽量让使用 git 的用户容易上手 zeta，同时也会对一些命令进行增强，比如很多 zeta 命令支持 `--json` 将输出格式化为 json，方便各种工具集成。

### 配置

```shell
zeta config --global user.email 'zeta@example.io'
zeta config --global user.name 'Example User'
```

### 检出存储库

使用 git 获取远程存储库的操作叫 `clone`（当然也可以用 `fetch`），在 zeta 中，我们限制其操作为 `checkout`，你也可以缩写为 `co`，以下是检出一个存储库：

```shell
zeta co http://zeta.example.io/group/repo xh1
zeta co http://zeta.example.io/group/repo xh1 -s dir1
```

### 修改、跟踪、提交

我们实现了类似 git 一样的 `status`、`add`、`commit` 命令，除了交互模式外，大体上是可用的，可以使用 `-h` 查看详细帮助，在正确设置了语言环境的系统中，zeta 会显示对应的语言版本。

```shell
echo "hello world" > helloworld.txt
zeta add helloworld.txt
zeta commit -m "Hello world"
```

### 推送和拉取

```shell
zeta push
zeta pull
```

## 特点

### 下载加速

支持 `direct`、`dragonfly`、`aria2` 三种加速器，通过 `core.accelerator` 或环境变量 `ZETA_CORE_ACCELERATOR` 配置。

| 加速器 | 说明 |
| :---: | --- |
| `direct` | 直接从 OSS 签名 URL 下载（AI 场景推荐） |
| `dragonfly` | 使用 dragonfly 集群 P2P 加速 |
| `aria2` | 使用 aria2c 多线程下载 |

```shell
zeta config --global core.accelerator direct
zeta config --global core.concurrenttransfers 8  # 并发下载数 (1-50)
```

### 逐一检出

逐个检出文件并立即释放 blob 对象，大仓库可节省 **60%+** 磁盘空间。

```shell
zeta co http://zeta.example.io/zeta-poc-test/zeta-poc-test --one
```

![](./docs/images/one-by-one.png)

### 按需获取

按需自动下载缺失对象（如 `zeta cat`、merge 场景）。禁用请设置 `ZETA_CORE_PROMISOR=0`。

### 稀疏检出

稀疏检出允许用户只检出存储库中的部分目录，而非完整的工作区。这对于巨型存储库特别有用：

```shell
# 检出指定目录
zeta co http://zeta.example.io/group/repo myrepo -s src/core -s src/utils
```

### 检出单个文件

我们在 zeta 中可以检出单个文件，只需要在 co 的过程中添加 `--limit=0` 意味着除了空文件其他文件均不检出，然后使用 zeta checkout -- path 检出相应的文件即可：

```shell
zeta co http://zeta.example.io/zeta-poc-test/zeta-poc-test --limit=0 z2
zeta checkout -- dev6.bin
```

### 更新部分文件

有些用户仅想修改部分文件，同样可以做到，使用**检出单个文件**检出特定的文件后，修改后执行：

```shell
zeta add test1/2.txt
zeta commit -m "XXX"
zeta push
```

### 拉取策略

HugeSCM 支持三种拉取策略：

- **merge** - 创建合并提交（默认）
- **rebase** - 将本地提交变基到远程分支之上
- **fast-forward only** - 仅允许快进合并

```shell
zeta pull                    # merge 策略（默认）
zeta pull --rebase           # rebase 策略
zeta pull --ff-only          # 仅快进合并
```

### 暂存功能

暂存功能允许临时保存工作进度：

```shell
zeta stash                   # 暂存所有修改
zeta stash save "WIP: 功能开发中"  # 带描述信息暂存
zeta stash list              # 列出所有暂存
zeta stash pop               # 应用并删除最近的暂存
```

### 分支切换

在不同分支或提交之间切换：

```shell
zeta switch feature          # 切换到分支
zeta switch -c new-feature   # 创建并切换到新分支
zeta switch abc123           # 切换到特定提交
```

### 将存储库从 Git 迁移到 HugeSCM

```shell
zeta-mc https://github.com/antgroup/hugescm.git hugescm-dev
```

## CDC（内容定义分片）

HugeSCM 引入了 CDC 用于高效处理大文件。与传统的固定大小分片不同，CDC 根据内容确定分片边界，实现更好的去重效果：

| 场景 | 固定分片 | CDC 分片 |
|------|---------|---------|
| 局部修改 | 所有后续分片改变 | 仅 1-2 个分片改变 |
| 增量同步 | 传输完整文件 | 仅传输变化分片 |
| 去重效果 | 低 | 高 |

启用 CDC 配置：

```toml
[fragment]
threshold = "1GB"      # 文件大小阈值
size = "1GB"           # 目标分片大小（固定分片）
enable_cdc = true      # 启用 CDC 分片
```

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
| `git clone` | `zeta checkout` (co) | 检出存储库，非全量克隆 |
| `git fetch` | `zeta pull --fetch` | 仅获取数据 |
| `git pull` | `zeta pull` | 拉取并合并 |
| `git switch` | `zeta switch` | 切换分支 |

## 额外的工具 - hot 命令

`hot` 是 Git 存储库维护工具，用于清理、迁移和优化 Git 存储库。

### 常见使用场景

| 任务 | 命令 |
|------|------|
| 查找大文件 | `hot size` / `hot smart -L20m` |
| 删除敏感数据 | `hot remove path/to/secret.txt --prune` |
| 迁移 SHA1 → SHA256 | `hot mc https://github.com/user/repo.git` |
| 清理过期引用 | `hot prune-refs "feature/deprecated-"` |
| 线性化历史 | `hot unbranch --confirm` |
| 查看对象 | `hot cat HEAD --json` |

完整文档见 [docs/hot.md](./docs/hot.md)。

## 许可证

Apache License Version 2.0, 请查看 [LICENSE](LICENSE)