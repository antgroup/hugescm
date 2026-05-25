# zeta status / add 大文件场景性能改造计划

## 1. 背景

`zeta status` 与 `zeta add` 在大仓库下被反映为"较慢"。zeta 的典型负载是
**少量大文件 / 大型二进制**（不是海量小文件），且 index 文件格式与 git 兼容、
不可改动。基于这两条约束，本计划只在内存数据结构、调用链和 IO 行为上做优化。

## 2. 关键调用链回顾

`Worktree.Status` → `status` → 两次比较：

1. `diffCommitWithStaging` → `diffTreeWithStaging`：`tree`（HEAD）vs `index`
   - `pkg/zeta/worktree_status.go:189-207`
   - 调用 `w.odb.Index()`
2. `diffStagingWithWorktree`：`index` vs `worktree`
   - `pkg/zeta/worktree_status.go:264-288`
   - 再次调用 `w.odb.Index()`，并构建 `filesystem.NewRootNode`

差异由 `merkletrie.DiffTreeContext` 驱动，对每对同名节点先调用
`doubleIter.sameHash`（`modules/merkletrie/doubleiter.go:139-150`）：

```go
if a.Mode() == b.Mode() && a.ModifiedAt().Equal(b.ModifiedAt()) {
    return true
}
return d.hashEqual(d.from.current, d.to.current)
```

只要快速路径未命中，就会触发 `hashEqual` → `Hash()` → 对 worktree 侧文件
**整文件 BLAKE3**（`modules/merkletrie/filesystem/node.go:203-216`），对大文件就是
GB 级 IO。

## 3. 真正的瓶颈

| 环节 | 大文件场景代价 | 备注 |
|---|---|---|
| `odb.Index()` 解码 | 低 | 大文件仓 entries 数量小 |
| `ReadPatterns` 递归 | 低 | 与文件大小无关 |
| `doCalculateHashForRegular` | **致命** | 一次 miss = 读 GB |
| `resolveFragmentsIndex` + `odb.Fragments` | 关键 | 大文件常走 fragments |
| `sameHash` 快速路径命中率 | **决定性** | 决定上面两项是否触发 |

## 4. 改造目标

1. **提高 `sameHash` 快速路径的命中率与正确性**，避免对未变化的大文件白白做
   全文件 BLAKE3。
2. **减少同一次 status 内重复的 IO 与解码**，把两次 `odb.Index()` 合并为一次。
3. **在快速路径意外失效时兜底**：通过 worktree-side cache 把 worktree 侧
   `Hash()` 短路成查表操作，仍然不读文件。

## 5. 不在范围内（明确不做）

- 修改 index 二进制格式（用户禁止）。
- `odb.Index()` 跨调用 LRU 缓存（大文件场景收益有限，且涉及 mutate / 并发风险）。
- `ignore.ReadPatterns` 缓存（与大文件无关，不在本期范围）。
- 并行 BLAKE3（前面三步落地后再用 profile 决定）。

## 6. 改造步骤

### 阶段 1：加固 `doubleIter.sameHash`

文件：`modules/merkletrie/doubleiter.go`

改动：

- 在比较 `Mode` 之后、调用 `hashEqual` 之前，新增一个 `noder.Sizer` 接口断言，
  当两侧都实现且 size 不同 → 直接返回 `false`（不触发文件读取）。
- 当 `ModifiedAt().Equal(...)` 失败时，再做一次"按微秒截断"的比较，规避
  filesystem 与 index `ModifiedAt`（`time.Unix(sec, nsec)`）的精度差。

新增接口：`modules/merkletrie/noder/noder.go`

```go
type Sizer interface {
    Size() int64
}
```

`filesystem.Node` 与 `mindex.Node` 已经各自暴露 `Size()`，只需对外通过 `Sizer`
接口让 `doubleiter` 类型断言可用。

风险：极低；逻辑严格更"严格"，只会减少误命中。

### 阶段 2：在 `status()` 内复用 idx

文件：`pkg/zeta/worktree_status.go`

改动：

- 新增内部方法 `diffTreeWithStagingFromIndex(ctx, t, idx, reverse)` 与
  `diffStagingWithWorktreeFromIndex(ctx, idx, cache, reverse, excludeIgnoredChanges)`，
  允许复用已经加载的 `*index.Index`。
- `status(ctx, hash)` 改为：先调用一次 `w.odb.Index()`，把 idx 同时传给两次
  比较。

收益：单次 status 减少一次完整 index 解码。

**重要约束（踩过的坑）**：复用的是 `*index.Index`，而 **不** 是
`mindex.NewRootNode(...)` 返回的 noder。`merkletrie.DiffTreeContext` 在
迭代过程中会原地 mutate 节点的 `children` 底层数组
（`frame.Drop` 把对应位置置为 nil，见
`modules/merkletrie/internal/frame/frame.go:86`），所以同一个 mindex
root 只能被消费一次；复用会在第二次 `frame.New → byName.Less` 时
nil-deref panic。

为锁死这一不变量，在 `modules/merkletrie/index/node_test.go` 加了
三个 sentinel 用例：
- `TestRootNodeIsConsumedDestructively`：证明一次 diff 后 root.children
  确实出现 nil；
- `TestRootNodeReuseAcrossDiffsPanics`：复现 panic；
- `TestFreshRootPerDiffIsSafe`：每次 diff 重建 root 是安全的——这正是
  `status()` 现在的做法。

兼容性：保留旧的 `diffCommitWithStaging` / `diffTreeWithStaging` /
`diffStagingWithWorktree` 公开签名，避免破坏其它调用点。

### 阶段 3：filesystem.Node 接入 worktree cache

目标：当 worktree 文件的 `(size, mtime, mode)` 与 index 中条目一致时，
直接复用 index 里的 hash，**完全跳过 `os.Open + BLAKE3`**。

#### 3.1 新增 cache

新文件：`modules/merkletrie/filesystem/cache.go`

```go
type CacheEntry struct {
    Size       int64
    ModifiedAt time.Time
    Hash       []byte // 36 字节：blob hash + mode bytes
}

type Cache struct {
    m map[string]CacheEntry // key: 相对仓库根的 slash 路径
}
```

`Cache` 只读，构造后不再修改，避免并发问题。

#### 3.2 把缓存灌进 worktree 侧 noder

新增 `filesystem.NewRootNodeWithCache(root, m, cache)` 入口；新建 `Node`
时把 `*Cache` 透传给所有子节点。

`Node.Hash()`：
1. 若节点已计算过（`len(n.hash) != 0`）→ 直接返回；
2. 否则查 cache：若命中且 `(size, mtime, mode)` 一致 → `n.hash = e.Hash`，
   返回；
3. 否则走原本的 `calculateHash`（即真正读文件）。

#### 3.3 在 `worktree_status.go` 中构造 cache

实现一个 `buildWorktreeCache(idx) *filesystem.Cache`：

- 遍历 `idx.Entries`；
- 对 fragments 条目使用 `resolveFragmentsIndex` 解出真实 hash 与 size；
- 把 `Name -> CacheEntry{Size, ModifiedAt, Hash || mode bytes}` 灌入。

随后 `diffStagingWithWorktreeFromIndex` 用这份 cache 构造
`filesystem.NewRootNodeWithCache`。

#### 3.4 与阶段 1 的关系

阶段 1 的 `sameHash` 加固是"先于 Hash 计算的快速路径"；阶段 3 的 cache
是"快速路径 miss 时仍能避免读文件的兜底"。两者互补：

- 95% 场景下大文件未变化，阶段 1 直接返回 true；
- mtime 真的轻微漂移导致快速路径 miss 时，阶段 3 可以让 worktree 侧 `Hash()`
  仍然命中 cache，从而 `hashEqual` 返回 true 而不读文件。

风险点：缓存 key 必须用 worktree 内的相对路径并与 `filesystem.Node.path`
统一。Symlink 与 fragments 路径在缓存时要明确区分；本次只为"普通 regular
文件"提供 cache，symlink 走原逻辑。命中后还会再校验一次 mode 字节，
防止 worktree 文件 kind 改变（如 Regular ↔ Executable）时复用旧 hash。

## 7. 验证

- `go build ./...` 通过；
- `go vet ./...` 通过；
- 已有的 `Status` / `Add` 单测：在改动模块上跑一遍现有测试集；
- 手动场景：在 1~5 GB 二进制为主的本地仓库上分别测 `zeta status` 三次，对比
  改造前后的 wall time。

## 8. 后续可做（不在本期）

- 并行 BLAKE3：**已评估，明确不做**。当前依赖的 `github.com/zeebo/blake3`
  没有公开的子树并行 API，BLAKE3 算法的多核并行需要内部
  chaining-value 接口；只能选择"换 hash 库"或"做 IO/hash 流水线"，前者
  涉及 `modules/plumbing/hash.go` 与 index 编解码至少 4 个文件的改造，
  后者只是把 wall time 压到 `max(IO, hash)`。结合 zeta 的实际场景：
  zeebo/blake3 已是 SIMD 加速实现，大文件场景下 IO 是真正瓶颈，引入
  并行的工程成本与收益不匹配，故不实施。
- `resolveFragmentsIndex` 在同一进程内的二级缓存（按 idx hash 失效）。
- 跨进程的 fsmonitor / inode 缓存（git core.fsmonitor 思路），需要单独设计。
