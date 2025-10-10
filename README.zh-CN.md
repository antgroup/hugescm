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

我们编写了 Inno Setup 脚本，可以使用 Docker + wine 在没有 Windows 的环境下生成安装包，可以运行 `amake/innosetup` 制作 Inno Setup 安装包 ：

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

## 额外的工具 - hot 命令

`hot` 命令是我们整合到 HugeSCM 中的一个 Git 存储库治理利器，它支持很多的场景：

+  你可以使用 `hot size（原始大小）`/`hot az（近似压缩大小）` 查看仓库中的大文件。
+  Git 存储库误提交了密码凭证等，可以使用 `hot remove` 删除并重写历史记录，`hot remove` 的重写速度特别快。
+  你也可以直接使用 `hot smart` 交互式操作删除仓库中的大文件，它结合了 `size, remove` 命令（如： `hot smart -L20m`）。
+  你可以使用 `hot mc` 将 Git 存储库的对象格式迁移到 `SHA256`，也可以从 `SHA256` 的迁移到 `SHA1`。
+  仓库无效分支标签太多，可以使用 `hot prune-refs（按前缀匹配）`/`hot expire-refs（按过期时间，是否合并）` 删除，亦可以使用 `hot scan-refs` 查看分支的情况。
+  你可以使用 `hot unbranch` 将存储库的历史线性化，也就是不包含任何合并点。
+  你亦可以使用 `hot unbranch -K1 master -Tnew-branch` 基于特定的版本创建一个孤儿分支，这将保留最近的历史，可用于开源或者重置历史场景。
+  你可以使用 `hot cat` 查看存储库中的文件，`commit/tree/tag/blob`，其中 `commit/tree/tag` 可以使用 `--json` 输出成 **JSON**，`blob` 则能智能的使用 16 进制输出二进制文件。

更多的帮助信息如下：

```txt
Usage: hot <command> [flags]

hot - Git 存储库维护工具

标志：
  -h, --help       显示上下文相关的帮助
  -V, --verbose    展示操作的更多细节
  -v, --version    展示版本信息并退出
      --debug      开启调试模式分析时间消耗

命令：
  cat            提供存储库对象的内容或类型和大小信息
  stat           查看存储库状态
  size           展示存储库体积和大文件
  remove         删除存储库中的文件并重写历史
  smart          交互模式清理存储库大文件
  graft          交互模式清理存储库大文件（嫁接模式）
  mc             迁移存储库对象格式到指定对象格式
  unbranch       线性化存储库历史
  prune-refs     清理指定前缀的引用
  scan-refs      扫描本地存储库中的引用
  expire-refs    清理过期引用
  snapshot       为工作树创建快照提交
  az             分析存储大文件
  co             EXPERIMENTAL: 将存储库克隆到新创建的目录中

运行 "hot <command> --help" 以获取有关命令的更多信息。
```

比如你查看仓库中的一张图片，可以这样做（二进制文件按照 16 进制展示）：

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
<img width="1253" height="905" alt="image" src="https://github.com/user-attachments/assets/3c84566a-9626-40e1-bffc-07ce2917c91a" />

## 许可证

Apache License Version 2.0, 请查看 [LICENSE](LICENSE)