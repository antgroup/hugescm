# Zeta 三方合并设计文档

> 英文版本：[merge-en.md](merge-en.md)

---

## 概述

本文档描述 Zeta (HugeSCM) 中三方合并（Three-Way Merge）的设计与实现。实现覆盖完整的合并流水线：Tree 级别的差异计算、重命名检测、文件/目录冲突解决，以及基于 diff3 算法的文本级三方合并。

文本合并层（`modules/diferenco`）是一个独立可复用的 Go 包，实现了 diff3 算法，支持多种 diff 后端（Histogram、Myers、ONP、Patience、Minimal）和多种冲突标记样式（merge、diff3、zdiff3）。同时支持非 UTF-8 文件的自动字符集检测与转码。

### 核心特性

- **纯 Go 实现的 diff3** — 不依赖外部 `git merge-file` 或 `diff3` 二进制（但支持作为可选外部驱动）
- **多种 diff 算法** — Histogram（默认）、Myers、ONP、Patience、Minimal、SuffixArray
- **多种冲突样式** — merge（默认）、diff3、zdiff3（zealous diff3）
- **字符集感知合并** — 自动检测并转码 GBK、Shift-JIS 等编码
- **Tree 级别合并** — 重命名检测、文件/目录冲突、文件模式冲突
- **二进制文件处理** — 内容嗅探 + 大小阈值（50 MiB）
- **可插拔架构** — 通过函数类型支持自定义合并驱动和文本解析器

---

## 架构总览

```
┌──────────────────────────────────────────────────────────────┐
│  应用层 (pkg/zeta/)                                          │
│  merge_tree.go — 命令入口、merge-base 解析、输出格式化        │
└──────────────────────────┬───────────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────────┐
│  Tree 合并层 (pkg/zeta/odb/)                                 │
│  merge.go       — 三方 tree 合并编排                          │
│  merge_driver.go — 文本合并分发 + 字符集还原                  │
│  merge_text.go  — 外部合并工具集成                            │
└──────────────────────────┬───────────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────────┐
│  文本合并层 (modules/diferenco/)                              │
│  merge.go    — diff3 算法、冲突检测与输出                     │
│  diferenco.go — diff 算法（Histogram、Myers、ONP 等）         │
│  text.go     — 字符集检测、二进制检测                         │
│  sink.go     — 行去重与索引                                   │
└──────────────────────────────────────────────────────────────┘
```

---

## 第一部分：文本级三方合并（`modules/diferenco`）

### 算法原理

文本合并使用经典的 **diff3** 算法，参考论文：

> Sanjeev Khanna, Keshav Kunal, and Benjamin C. Pierce.
> "A Formal Investigation of Diff3." FSTTCS 2007.
> http://www.cis.upenn.edu/~bcpierce/papers/diff3-short.pdf

实现源自 [node-diff3](https://github.com/bhousel/node-diff3)（JavaScript）→ [epiclabs-io/diff3](https://github.com/epiclabs-io/diff3)（Go 移植），并做了大量增强。

#### 执行步骤

1. **行索引化**：将文本按行拆分，通过 `Sink` 结构去重。每个唯一行映射为整数索引，使 diff 算法在 `[]int` 上操作而非 `[]string`。

2. **两路 diff**：分别计算 `diff(O, A)` 和 `diff(O, B)`，使用选定的算法（默认 Histogram）。

3. **Hunk 合并**：将两组 diff 结果合并到 O 的统一时间线上。来自双方的重叠 hunk 表示潜在冲突。

4. **冲突检测**：对每个重叠区域，检查 A 和 B 是否做了相同修改（消除假冲突）。真正的冲突记录其 A/O/B 范围。

5. **输出生成**：非冲突区域直接输出。冲突按选定样式格式化为冲突标记。

### 冲突样式

```go
const (
    STYLE_DEFAULT      = iota  // <<<<<<< / ======= / >>>>>>>
    STYLE_DIFF3                // <<<<<<< / ||||||| / ======= / >>>>>>>
    STYLE_ZEALOUS_DIFF3        // 最小化 A/B hunk + 完整 O 上下文
)
```

**默认（merge）样式**：
```
<<<<<<< ours
ours 侧的修改行
=======
theirs 侧的修改行
>>>>>>> theirs
```

**diff3 样式**：在 `|||||||` 标记之间显示 base 版本。

**zdiff3（zealous diff3）样式**：类似 diff3，但将 A/B hunk 的公共前缀/后缀提取到标记外部，最小化冲突区域。

### API

```go
// 高级接口：使用默认选项（Histogram 算法、merge 样式）
func DefaultMerge(ctx context.Context, o, a, b string, labelO, labelA, labelB string) (string, bool, error)

// 完全控制：指定算法和冲突样式
func Merge(ctx context.Context, opts *MergeOptions) (string, bool, error)

// 快速检查：合并这些文本是否会产生冲突？
func HasConflict(ctx context.Context, textO, textA, textB string) (bool, error)
```

### Diff 算法

| 算法 | 描述 | 适用场景 |
|------|------|---------|
| Histogram | 基于频率的 LCS（类似 JGit） | 通用（默认） |
| Myers | 经典 O(ND) 算法 | 小规模 diff |
| ONP | Wu 等人的 O(NP) 算法 | 均衡编辑 |
| Patience | Patience diff（唯一行匹配） | 大量重复行的代码 |
| Minimal | 有界双向 Myers | 最小编辑脚本 |
| SuffixArray | 基于后缀数组的 LCS | 大文件且有重复 |

### 二进制检测

文件通过两种启发式方法判定为二进制：
1. **NUL 字节扫描**：检查前 8000 字节是否包含 NUL 字节（与 Git 相同）
2. **大小阈值**：超过 100 MiB 的文件直接拒绝（`MAX_DIFF_SIZE`）

### 字符集处理

启用 `textconv` 时：
1. 对前 8000 字节进行 MIME 类型检测
2. 从 MIME 参数中提取字符集
3. 转码为 UTF-8 进行 diff/merge 操作
4. 合并完成后，将结果编码回原始字符集（ours 侧的字符集）

支持的字符集包括 UTF-8、GBK、GB18030、Shift-JIS、EUC-KR、ISO-8859-*、Windows-125* 等。

---

## 第二部分：Tree 级三方合并（`pkg/zeta/odb`）

### 入口函数

```go
func (d *ODB) MergeTree(ctx context.Context, o, a, b *object.Tree, opts *MergeOptions) (*MergeResult, error)
```

参数：
- `o` — ancestor（merge-base）tree
- `a` — "ours" tree
- `b` — "theirs" tree
- `opts` — 合并配置

### 合并流水线

#### 步骤 1：计算差异

```go
func (d *ODB) mergeDifferences(ctx context.Context, o, a, b *object.Tree) (*differences, error)
```

分别执行 `DiffTree(O, A)` 和 `DiffTree(O, B)`，启用精确重命名检测。结果合并到统一的 `differences` 结构中：

```go
type differences struct {
    entries map[string]*ChangeEntry  // 路径 → {Ancestor, Our, Their}
    renames map[string]*RenameEntry  // 原路径 → 重命名信息
    ours    map[string]bool          // ours 侧修改的路径集合
    theirs  map[string]bool          // theirs 侧修改的路径集合
}
```

`overrideOur` 先处理 ours 变更，然后 `overrideTheir` 将 theirs 变更合并到已有条目中。这种两遍处理方式正确处理了双方修改同一文件的情况。

#### 步骤 2：检测重命名冲突

对每个重命名条目，检查双方是否将同一文件重命名到不同目标：

```go
func (e *RenameEntry) conflict() bool {
    return e.Our != nil && e.Their != nil && !e.Our.Equal(e.Their)
}
```

如果是，报告 `CONFLICT (rename/rename)`。

#### 步骤 3：检测文件/目录名冲突

```go
func (d *differences) nameConflicts() map[string]string
```

检测一方添加文件 `foo` 而另一方添加目录 `foo/` 的情况。解决方式：将文件重命名为 `foo~<branch_name>` 并报告 `CONFLICT (file/directory)`。

#### 步骤 4：快速路径解决

对每个变更条目，尝试无需文本合并的解决：

```
Ancestor == Our   → 采用 Theirs（ours 未修改）
Ancestor == Their → 采用 Ours（theirs 未修改）
Our == Their      → 采用任一方（双方做了相同修改）
其他              → 需要文本合并
```

#### 步骤 4.1：合并侧决策细节

以下是 `MergeTree` 主循环中对每个 `ChangeEntry` 的完整决策树：

```
对于 entries 中的每个条目 e:
│
├─ e.Ancestor == e.Our（ours 未修改）
│   ├─ e.Their != nil → 采用 Their 版本（theirs 的修改生效）
│   └─ e.Their == nil → 删除该文件（theirs 删除了，ours 没动）
│
├─ e.Ancestor == e.Their（theirs 未修改）
│   ├─ e.Our != nil → 采用 Our 版本（ours 的修改生效）
│   └─ e.Our == nil → 删除该文件（ours 删除了，theirs 没动）
│
├─ e.Our == e.Their（双方做了相同修改）
│   ├─ e.Our != nil → 采用 Our 版本（两边一样，取任一方）
│   └─ e.Our == nil → 删除该文件（双方都删除了）
│
└─ 其他（真正的冲突，进入 mergeEntry）
    │
    ├─ e.Ancestor == nil（双方新增同一路径）
    │   ├─ Our.Hash == Their.Hash → CONFLICT(distinct modes)，保留 ours
    │   ├─ 二进制/Fragments/大文件 → CONFLICT(binary)，保留 ours
    │   └─ 文本文件 → 以空 blob 为 base 做三方合并
    │       ├─ 合并成功 → 采用合并结果
    │       └─ 有冲突 → CONFLICT(add/add)，结果含冲突标记
    │
    ├─ e.Our != nil && e.Their != nil（双方都修改了）
    │   ├─ Our.Hash == Their.Hash → CONFLICT(distinct modes)，保留 ours
    │   ├─ 二进制/Fragments/大文件 → CONFLICT(binary)，保留 ours
    │   └─ 文本文件 → 以 Ancestor 为 base 做三方合并
    │       ├─ 合并成功，无模式冲突 → 采用合并结果 + 新模式
    │       ├─ 合并成功，有模式冲突 → CONFLICT(distinct modes)
    │       └─ 有内容冲突 → CONFLICT(content)，结果含冲突标记
    │
    └─ e.Our == nil || e.Their == nil（一方删除，另一方修改）
        ├─ Our == nil → CONFLICT(modify/delete): ours 删除，theirs 修改
        └─ Their == nil → CONFLICT(modify/delete): theirs 删除，ours 修改
        └─ 保留修改方的版本
```

**模式（filemode）决策逻辑**：

当双方都修改了文件且内容合并成功时，还需要决定最终的文件模式：

```
Ancestor.Mode == Our.Mode  → 采用 Their.Mode（ours 没改模式）
Ancestor.Mode == Their.Mode → 采用 Our.Mode（theirs 没改模式）
Our.Mode != Their.Mode     → CONFLICT(distinct modes)，采用 Our.Mode
Our.Mode == Their.Mode     → 采用 Our.Mode（双方改成了一样的模式）
```

**冲突时的默认保留策略**：

所有无法自动解决的冲突，结果 tree 中保留的文件版本遵循以下规则：

| 冲突类型 | 结果 tree 中保留的版本 |
|----------|----------------------|
| 内容冲突（content） | 合并后的文本（含 `<<<<<<<` 冲突标记） |
| 二进制冲突（binary） | ours 侧的文件 |
| 模式冲突（distinct modes） | ours 侧的文件 |
| 修改/删除冲突（modify/delete） | 修改方的文件（无论是 ours 还是 theirs） |
| 文件/目录冲突（file/directory） | 文件重命名为 `path~branch_name` |
| 重命名冲突（rename/rename） | 双方各自的重命名保留 |

**重命名处理的决策逻辑**：

```
对于 renames 中的每个条目 e:
│
├─ e.Our == nil（仅 theirs 重命名）
│   → 无冲突，theirs 的重命名生效
│
├─ e.Their == nil（仅 ours 重命名）
│   → 无冲突，ours 的重命名生效
│
├─ e.Our.Path == e.Their.Path（双方重命名到相同目标）
│   → 无冲突，采用任一方
│
└─ e.Our.Path != e.Their.Path（双方重命名到不同目标）
    → CONFLICT(rename/rename)
```

**文件/目录名冲突的决策逻辑**：

```
对于 nameConflicts 中的每个冲突 (file_path, dir_path):
│
├─ file_path 来自 theirs 侧
│   → 将文件重命名为 file_path~<Branch2>
│
└─ file_path 来自 ours 侧
    → 将文件重命名为 file_path~<Branch1>
│
└─ 原路径从 entries 中删除，新路径加入 entries
└─ 目录下的文件正常保留不受影响
```

#### 步骤 5：文本合并（mergeEntry）

对需要合并的条目，按场景分类处理：

| 场景 | 处理方式 |
|------|---------|
| 双方新增（ancestor=nil），hash 相同 | 仅模式冲突 |
| 双方新增，二进制/fragments/大文件 | 二进制冲突，保留 ours |
| 双方新增，文本 | 以空 blob 为 base 合并 |
| 双方修改，hash 相同 | 仅模式冲突 |
| 双方修改，二进制/大文件 | 二进制冲突，保留 ours |
| 双方修改，文本 | 三方文本合并 |
| 一方删除，另一方修改 | modify/delete 冲突 |

#### 步骤 6：构建结果 Tree

收集所有已解决的条目，通过 `treeMaker.makeTrees()` 构建新的 tree 对象。

### 可插拔组件

#### MergeDriver

```go
type MergeDriver func(ctx context.Context, o, a, b string, labelO, labelA, labelB string) (string, bool, error)
```

执行实际文本合并的函数。默认：`diferenco.DefaultMerge`。可替换为：
- `odb.ExternalMerge` — 调用外部 `git merge-file`
- `odb.Diff3Merge` — 调用外部 `diff3`
- 自定义驱动（如 XML 合并、JSON 合并）

#### TextResolver

```go
type TextResolver func(ctx context.Context, oid plumbing.Hash, textconv bool) (string, string, error)
```

根据 blob OID 读取文本内容，返回文本内容 + 检测到的字符集。默认实现从本地存储读取；在部分克隆场景下，可触发缺失对象的按需拉取。

### 冲突类型

| 常量 | 说明 |
|------|------|
| `CONFLICT_CONTENTS` | 文本内容冲突 |
| `CONFLICT_BINARY` | 二进制文件冲突 |
| `CONFLICT_FILE_DIRECTORY` | 文件与目录同名冲突 |
| `CONFLICT_DISTINCT_MODES` | 文件模式（权限）冲突 |
| `CONFLICT_MODIFY_DELETE` | 一方修改一方删除 |
| `CONFLICT_RENAME_RENAME` | 双方重命名到不同目标 |
| `CONFLICT_RENAME_COLLIDES` | 重命名目标冲突 |
| `CONFLICT_RENAME_DELETE` | 一方重命名一方删除 |

### MergeResult

```go
type MergeResult struct {
    NewTree   plumbing.Hash  // 合并后的新 tree OID（即使有冲突也会生成）
    Conflicts []*Conflict    // 冲突列表（为空表示干净合并）
    Messages  []string       // 人类可读的合并消息
}
```

合并后的 tree 始终会生成（冲突标记嵌入文本文件中），类似 `git merge-tree`。调用方通过 `len(result.Conflicts) > 0` 判断是否需要手动解决。

---

## 第三部分：Merge-Base 解析

当存在多个 merge-base 时（交叉合并），Zeta 递归合并它们：

```go
func (r *Repository) resolveAncestorTree0(ctx context.Context, into, from *object.Commit, ...) (*object.Tree, error) {
    bases, _ := into.MergeBase(ctx, from)
    switch len(bases) {
    case 0:
        return r.odb.EmptyTree(), nil  // 无关历史
    case 1:
        return bases[0].Root(ctx)      // 单一 merge-base
    default:
        // 递归：先合并 merge-base 们
        return r.resolveAncestorTree0(ctx, bases[0], bases[1], ...)
    }
}
```

这与 Git 的 "recursive" 合并策略行为一致。

---

## 第四部分：与 Git 的差异

| 特性 | Git (merge-ort) | Zeta |
|------|----------------|------|
| 重命名检测 | 模糊匹配（相似度评分） | 仅精确匹配 |
| 合并策略 | ours、theirs、octopus、subtree | 仅默认策略 |
| 冲突样式 | merge、diff3、zdiff3 | merge、diff3、zdiff3 |
| Diff 算法 | Myers、Histogram、Patience、Minimal | Histogram、Myers、ONP、Patience、Minimal、SuffixArray |
| 大文件处理 | Git LFS（外部） | 内置 Fragments 对象检测 |
| 字符集处理 | 无（假设 UTF-8） | 自动检测并转码 |
| 二进制阈值 | 仅基于内容检测 | 50 MiB 大小阈值 + 内容检测 |
| 哈希算法 | SHA-1 / SHA-256 | BLAKE3 |
| 实现语言 | C (libgit2) / C (git) | 纯 Go |

---

## 第五部分：使用示例

### 程序调用（Go）

```go
import (
    "context"
    "code.alipay.com/zeta/zeta/modules/diferenco"
    "code.alipay.com/zeta/zeta/pkg/zeta/odb"
)

// 文本级合并（独立使用，不需要存储）
merged, hasConflict, err := diferenco.DefaultMerge(ctx,
    baseText, oursText, theirsText,
    "base", "ours", "theirs",
)

// 文本级合并（完全控制选项）
merged, hasConflict, err := diferenco.Merge(ctx, &diferenco.MergeOptions{
    TextO:  baseText,
    TextA:  oursText,
    TextB:  theirsText,
    LabelO: "base",
    LabelA: "ours",
    LabelB: "theirs",
    A:      diferenco.Histogram,
    Style:  diferenco.STYLE_ZEALOUS_DIFF3,
})

// Tree 级合并（需要 ODB 进行 blob 存储）
result, err := db.MergeTree(ctx, ancestorTree, oursTree, theirsTree, &odb.MergeOptions{
    Branch1:      "main",
    Branch2:      "feature",
    Textconv:     true,
    MergeDriver:  diferenco.DefaultMerge,
    TextResolver: readBlobText,
})
if len(result.Conflicts) > 0 {
    // 处理冲突
    for _, c := range result.Conflicts {
        fmt.Printf("CONFLICT: %s (type=%d)\n", c.Our.Path, c.Types)
    }
}
// result.NewTree 为合并后的 tree OID
```

### 命令行

```shell
# 三方合并两个分支的 tree
zeta merge-tree branch1 branch2

# 指定 merge-base
zeta merge-tree --merge-base=base branch1 branch2

# JSON 格式输出（便于工具集成）
zeta merge-tree --json branch1 branch2

# 冲突后：手动解决或强制选择某一方
zeta checkout <rev> -- <file>
```

---

## 源文件索引

| 文件 | 层级 | 职责 |
|------|------|------|
| `modules/diferenco/merge.go` | 文本层 | diff3 算法、冲突标记、合并样式 |
| `modules/diferenco/diferenco.go` | 文本层 | Diff 算法（Histogram、Myers、ONP 等） |
| `modules/diferenco/text.go` | 文本层 | 字符集检测、二进制检测、转码 |
| `modules/diferenco/sink.go` | 文本层 | 行去重与整数索引 |
| `pkg/zeta/odb/merge.go` | Tree 层 | Tree 级合并编排、冲突检测 |
| `pkg/zeta/odb/merge_driver.go` | Tree 层 | 文本合并分发、字符集还原 |
| `pkg/zeta/odb/merge_text.go` | Tree 层 | 外部合并工具集成（git merge-file、diff3） |
| `pkg/zeta/merge_tree.go` | 应用层 | 命令入口、merge-base 解析、输出格式化 |

---

## 参考资料

- [A Formal Investigation of Diff3](http://www.cis.upenn.edu/~bcpierce/papers/diff3-short.pdf) — Khanna, Kunal, Pierce (2007)
- [Merging with Diff3](https://blog.jcoglan.com/2017/05/08/merging-with-diff3/) — James Coglan
- [node-diff3](https://github.com/bhousel/node-diff3) — 原始 JavaScript 实现
- [epiclabs-io/diff3](https://github.com/epiclabs-io/diff3) — Go 移植版
- [Git merge-ort](https://git-scm.com/docs/git-merge) — Git 的合并策略（对比参考）
