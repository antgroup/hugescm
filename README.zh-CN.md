# HugeSCM - 基于云的下一代版本控制系统

[![license badge](https://img.shields.io/github/license/antgroup/hugescm.svg)](LICENSE)
[![Master Branch Status](https://github.com/antgroup/hugescm/workflows/CI/badge.svg)](https://github.com/antgroup/hugescm/actions)
[![Latest Release Downloads](https://img.shields.io/github/downloads/antgroup/hugescm/latest/total.svg)](https://github.com/antgroup/hugescm/releases/latest)
[![Total Downloads](https://img.shields.io/github/downloads/antgroup/hugescm/total.svg)](https://github.com/antgroup/hugescm/releases)
[![Version](https://img.shields.io/github/v/release/antgroup/hugescm)](https://github.com/antgroup/hugescm/releases/latest)

[English](./README.md)

## 概述

HugeSCM（内部代号 zeta）是一种基于云的下一代版本控制系统，旨在解决研发过程中存储库规模问题。它既能处理单一存储库体积巨大的挑战，也能应对存储单一文件巨大的问题。相比于传统的集中式版本控制系统（如 Subversion ）和传统的分布式版本控制系统（如 Git ），HugeSCM 不受存储架构和传输协议的限制。随着研发活动的推进，传统的版本控制系统已经无法满足巨型存储库的需求，这就是 HugeSCM 诞生的原因。
HugeSCM 是一种数据分离的版本控制系统，目录结构，提交记录，分支信息存储在分布式数据库中，而文件内容则存储在分布式文件系统或者对象存储中。国内外的开发者曾将 Git 对象存储到 OSS /分布式文件系统中，对 git 架构进行改造，但效果非常差。HugeSCM 需要吸取这些教训，对其架构进行精心设计，避免因存储数据到 DB/OSS 带来的性能下降问题。
HugeSCM 适合单一大库研发，特别是 AI 大模型研发，以及游戏研发，驱动开发等场景。

HugeSCM 主要通过以下方式实现解决存储库规模问题：
+  数据分离原则：HugeSCM 采用数据分离的原则，将版本控制系统的数据分为元数据和文件数据，按照不同的策略存储，解决了单机文件存储的上限。
+  高效传输协议：HugeSCM 采用高效的传输协议，通过优化数据传输过程，减少数据传输的时间和带宽消耗。这使得 HugeSCM 能够快速而可靠地处理大规模存储库的版本控制操作。
+  先进的算法和数据结构：HugeSCM 使用先进的算法和数据结构来组织和管理存储库的数据。这些算法和数据结构能够有效地处理大规模存储库的存储和检索需求，提高操作的效率和性能。HugeSCM 引入了 fragments 对象，解决了单一文件的规模问题。这意味着 HugeSCM 除了可以存储源代码，还可以方便的存储二进制数据，AI 模型，二进制依赖等等。
通过以上策略和技术，HugeSCM 能够有效地解决存储库规模问题，提供高性能、可靠和灵活的版本控制服务。

**它吸取了 Git 的经验，摆脱了 Git 的历史包袱，总之我们感谢这些前辈。**

## 技术细节

对象格式：[object-format.md](./docs/object-format.md)  
传输协议：[protocol.md](./docs/protocol.md)

## 构建

开发者安装好最新版本的 Golang 程序后，就可以构建 HugeSCM 客户端，可以选择安装 make 或者 [bali](https://github.com/balibuild/bali)（构建打包工具）。

```sh
bali -T windows
# create rpm,deb,tar,sh pack
bali -T linux -A amd64 --pack='rpm,deb,tar,sh'
```

bali 构建工具可以制作 `zip`, `deb`, `tar`, `rpm`, `sh (STGZ)` 压缩/安装包。

### Windows 安装包

我们编写了 Inno Setup 按照包脚本，可以使用 Docker + wine 在没有 Windows 的环境下生成安装包，可以运行 `amake/innosetup` 制作 Inno Setup 安装包 ：

```shell
docker run --rm -i -v "$TOPLEVEL:/work" amake/innosetup xxxxx.iss
```

然后即可生成安装包，在此之前我们需要运行 `bali --target=windows --arch=amd64` 先构建 Windows 平台二进制出来。

注意：在搭载 Apple Silicon 芯片的 macOS 机器上，可以使用 Orbstack 开启 Rosetta 运行该镜像制作 Windows 安装包。

## 使用

用户可以运行 `zeta -h` 查看 zeta 所有命令，并运行 `zeta ${command} -h` 查看命令详细帮助，我们尽量让使用 git 的用户容易上手 zeta，同时也会对一些命令进行增强，比如很多 zeta 命令支持 `--json` 将输出格式化为 json，方便各种工具集成。

### 配置

```shell
zeta config --global user.email 'zeta@example.io'
zeta config --global user.name 'Example User'
```

### 检出存储库

使用 git 获取远程存储库的操作叫 `clone`（当然也可以用 `fetch`）, 在 zeta 中，我们限制其操作为 `checkout`，你也可以缩写为 `co`，以下是检出一个存储库：

```shell
zeta co http://zeta.example.io/group/repo xh1
zeta co http://zeta.example.io/group/repo xh1 -s dir1
```

### 修改，跟踪，提交

我们实现了类似 git 一样的 `status`,`add`,`commit` 命令，除了交互模式外，大体上是可用的，可以使用 `-h` 查看详细帮助，在正确设置了语言环境的系统中，zeta 会显示对应的语言版本。

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

目前 zeta 客户端支持三种类型的加速，可以通过修改配置 `core.accelerator`或者覆盖环境变量`ZETA_CORE_ACCELERATOR`，支持的值有 `direct`，`dragonfly`，`aria2`。

| 加速器 | 加速逻辑 | 备注 |
| :---: | --- | --- |
| `direct` | 由 zeta 客户端获取签名地址直接从 OSS 下载，流量不经 Zeta Server | 像 AI 场景可以使用该机制，实际上 oss 走签名下载后，速度是足够的。相反，如果未设置加速器，下载网速可能达不到理想状态，因此，用户应当尽量开启直连（当无法访问 oss 签名 url 时，则不应设置）。|
| `dragonfly` | 调用 dragonfly 客户端 dfget 下载，dfget 能使用 dragonfly 集群能力。 | 可以使用 `ZETA_EXTENSION_DRAGONFLY_GET`指定 dfget 路径，而不是使用 PATH 中的 dfget。  |
| `aria2` | 调用 aria2c 命令行下载，aria2 是业内著名的下载工具。 | 可以使用 `ZETA_EXTENSION_ARIA2C`指定 aria2c 路径，而不是使用 PATH 中的 aria2c。  |

```shell
# 开启 oss 直连下载
zeta config --global core.accelerator direct
```

默认情况下，无论是开启了加速器还是未开启加速器，我们都未开启并行下载支持，用户可以配置 `core.concurrenttransfers`（环境变量 `ZETA_CORE_CONCURRENT_TRANSFERS`）设置并发下载数，有效值是 `1-50`，当你开启了并发下载，你会发现收益可能并不是那么明显，这通常会受带宽影响。

### 逐一检出

以前 Git LFS 在下载模型时，模型文件总容量为 100GB 的仓库可能需要超过 300GB 的存储空间，其中模型文件 100GB，分割后的文件 100GB，Git LFS 对象（`.git/lfs/objects/xx/...`）100GB，很多时候如果磁盘空间不够，将无法下载模型，在 zeta 中我们引入了 `zeta co url --one` 机制，检出一个文件立即删除相应的 blob 对象，这样 100GB 的模型文件，最终占用的空间可能也就 100GB 多一点，空间节省 60% 以上。

```shell
zeta co http://zeta.example.io/zeta-poc-test/zeta-poc-test --one
```

![](./docs/images/one-by-one.png)

我们在设计对象模型时还为 TreeEntry 增加了 `Size` 字段，这使得我们可以很容易实现一个 API 预估模型存储库检出所需空间大小，结合逐一检出机制，能够很好的节省磁盘空间。

### 按需获取

zeta 在设计之初是仅支持下载特定 commit 相关的对象，也支持 checkout 后删除相关的 blob，但事实证明用户的场景是复杂的，我们为 `zeta cat`引入了自动下载缺失的对象，截图如下（不确定对象大小，可以使用 zeta cat --limit 或者 zeta cat -s 避免大文件输出到终端）。

如要禁用自动下载缺失的对象，可设置环境变量 `ZETA_CORE_PROMISOR=0`禁用该功能。此外，我们还为 merge 等场景引入了自动下载缺失的对象功能。

### 检出单个文件

我们在 zeta 中可以检出单个文件，只需要在 co 的过程中添加 `--limit=0`意味着除了空文件其他文件均不检出，然后使用 zeta checkout -- path 检出相应的文件即可：

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

### 将存储库从 Git 迁移到 HugeSCM

```shell
zeta-mc https://github.com/antgroup/hugescm.git hugescm-dev
```

## Hot 命令

`hot` 命令是一个有去的 Git 存储库维护工具，它不仅支持删除存储库中的文件并重写历史（如大文件，密码文件等：`hot remove`），还支持分析存储库有哪些大文件（原始大小：`hot size`，压缩大小：`hot az`）,还支持友好的查看文件内容（`hot cat`），删除无效的分支，标签（按前缀删除：`hot prune-refs`，按过期时间或已合并删除：`hot expire-refs`），还支持查看存储库状态（`hot stat`），更多的命令可以查看帮助信息：

```txt
Usage: hot <command> [flags]

hot - Git repositories maintenance tool

Flags:
  -h, --help       Show context-sensitive help
  -V, --verbose    Make the operation more talkative
  -v, --version    Show version number and quit
      --debug      Enable debug mode; analyze timing

Commands:
  cat            Provide contents or details of repository objects
  stat           View repository status
  size           Show repositories size and large files
  remove         Remove files in repository and rewrite history
  smart          Interactive mode to clean repository large files
  graft          Interactive mode to clean repository large files (Grafting mode)
  mc             Migrate a repository to the specified object format
  unbranch       Linearize repository history
  prune-refs     Prune refs by prefix
  scan-refs      Scan references in a local repository
  expire-refs    Clean up expired references
  snapshot       Create a snapshot commit for the worktree
  az             Analyze repository large files
  co             EXPERIMENTAL: Clones a repository into a newly created directory

Run "hot <command> --help" for more information on a command.
```

比如你查看仓库中的一张图片，可以这样做：

```shell
hot cat HEAD:docs/images/blob.png
```

<img width="1253" height="814" alt="image" src="https://github.com/user-attachments/assets/fe1d7e8d-c511-4deb-b5f1-9cc4c082a36d" />

比如你查看仓库的信息，可以这样做：

```shell
hot stat
```

<img width="1253" height="814" alt="image" src="https://github.com/user-attachments/assets/b585dab7-38fd-490f-b178-98ab56205f8f" />

将 Git 存储库对象格式从 SHA1 迁移到 SHA256：

```shell
hot mc https://github.com/antgroup/hugescm.git
```
<img width="1253" height="905" alt="image" src="https://github.com/user-attachments/assets/3e15e5de-e297-4a3a-9a33-0a361e860486" />

## 许可证

Apache License Version 2.0, 请查看 [LICENSE](LICENSE)