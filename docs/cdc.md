# CDC (Content-Defined Chunking) 实现文档

## 一、核心原理

### 传统固定分片的问题

传统 VCS 使用**固定大小分片**,存在严重缺陷:

```
文件版本 1: [AAAAA][BBBBB][CCCCC][DDDDD]
                 ↑ 插入一个字节
文件版本 2: [AXAAAA][BBBBB][CCCCC]  ← 所有分片边界偏移!
```

**结果**: 仅仅插入 1 字节,导致所有后续分片都改变,去重率接近 0%。

### CDC 的解决方案

CDC 通过**内容决定边界**,而不是固定偏移:

```
文件版本 1: [AAAAA][BBBBB][CCCCC][DDDDD]
                 ↑ 插入一个字节
文件版本 2: [AX][AAAAA][BBBBB][CCCCC][DDDDD]
            ↑ 只有这一个分片改变
```

**结果**: 局部修改只影响附近的 1-2 个分片,其他分片保持不变。

---

## 二、FastCDC 算法实现

### 核心算法

我们使用 **FastCDC** 算法,这是工业级的 CDC 实现:

```go
// FastCDC 滚动哈希 (Gear Hash)
hash = (hash << 1) + gearTable[byte]
```

### 归一化切割策略

FastCDC 的核心创新:根据当前分片大小动态调整切割概率

```
阶段 1 (0 ~ minSize): 不切割
阶段 2 (minSize ~ normalSize): 使用 maskShort (更多 bits → 更难切割,让小分片继续增长)
阶段 3 (normalSize ~ normalSize+normalSpan): 使用 maskNormal (标准切割概率)
阶段 4 (normalSize+normalSpan ~ maxSize): 使用 maskLong (更少 bits → 更容易切割,关闭过大分片)
阶段 5 (maxSize+): 强制切割
```

**三 mask 策略**:
- `maskShort = 2^(maskBits+1) - 1`: 更多 bits,更难切割 (让小分片继续增长到 normalSize)
- `maskNormal = 2^maskBits - 1`: 标准切割概率
- `maskLong = 2^(maskBits-2) - 1`: 更少 bits,更容易切割 (防止分片过大)

其中 `maskBits = clamp(log2(targetSize) - 1, 10, 24)`。

### 参数配置

**默认参数**:

```go
targetSize = Fragment.Size()       // 默认 1GB (可通过配置覆盖)
minSize    = max(target/4, 64KiB)  // 最小分片,下限 64KiB
maxSize    = min(target*8, 64MiB)  // 最大分片,上限 64MiB
normalSize = target                // short/normal 阶段分界
normalSpan = target                // normal 阶段长度
```

**AI 模型场景推荐配置 (targetSize = 4MB)**:

AI 模型文件的特点:
- 典型张量大小: 几 MB 到几百 MB
- Fine-tuning 更新: 通常是整个张量或较大区域
- Checkpoint 文件: 10GB - 100GB

当配置 `fragment.size = "4m"` 时:

| 指标 | 1MB 分片 | 4MB 分片 | 改进 |
|------|---------|---------|------|
| 10GB 模型 | ~10000 fragments | ~2500 fragments | 减少 75% 元数据 |
| 去重效果 | 优秀 | 优秀 (相近) | 保持高去重率 |
| CPU 开销 | 高 | 低 | 减少 hash 计算次数 |
| 传输协商 | 慢 | 快 | metadata 传输更快 |

---

## 三、流式处理实现

### Rolling Buffer 架构

CDC 需要检测边界后才能处理分片,无法实现真正的纯 streaming:

```
1. 读取字节计算 rolling hash
2. 检测到边界
3. 然后才能哈希分片数据
```

问题:流一旦读过去就无法"回退"。

**解决方案**:使用滚动缓冲区 (Rolling Buffer)

```go
// 缓冲区大小 = maxSize
chunkBuf := make([]byte, 0, c.maxSize)

// 检测到边界后通过回调传递数据
onChunk(span Span, data io.Reader)
```

**内存占用**: O(maxSize)
- 典型值: 64MiB (maxSize 上限)
- 与文件大小无关,只与分片大小有关

**这是工业标准做法**:
- restic: 使用滚动缓冲区
- borg: 使用滚动缓冲区
- rsync: 使用滚动缓冲区

### Pipeline 设计

**单遍扫描,零临时文件**:

```go
func (r *Repository) writeCDCFragments(ctx context.Context, reader io.Reader, size int64) (oid plumbing.Hash, fragments bool, err error) {
    // 1. 计算完整文件哈希
    h := plumbing.NewHasher()
    tr := io.TeeReader(reader, h)

    // 2. 创建 CDC 分片器
    chunker := NewChunker(r.Fragment.Size())

    // 3. 单遍流式分片 + 哈希计算
    index := uint32(0)
    walkErr := chunker.Walk(tr, func(span Span, data io.Reader) error {
        chunkHash, hashErr := r.odb.HashTo(ctx, data, span.Size)
        if hashErr != nil {
            return hashErr
        }
        ff.Entries = append(ff.Entries, &object.Fragment{
            Index: index,
            Hash:  chunkHash,
            Size:  uint64(span.Size),
        })
        index++
        return nil
    })

    // 4. 保存 Fragments 对象
    ff.Origin = h.Sum()
    oid, err = r.odb.WriteEncoded(ff)
    return
}
```

**优点**:
- 单 pass
- full hash + chunk hash 同时算
- 无临时文件

### 尾部合并

Walk 方法实现了尾部合并逻辑:如果最后一个分片小于 minSize,会将其合并到前一个分片中,避免产生过小的 fragment 元数据。实现方式是延迟一个 chunk 的 emit,直到确认下一个 chunk 不需要合并。

---

## 四、配置使用

### 启用 CDC

在 `.zeta/zeta.toml` 文件中添加:

```toml
[fragment]
enable_cdc = true          # 启用 CDC 分片 (Boolean 类型,支持配置 merge)
```

**配置说明**:
- `enable_cdc` 是 `Boolean` 类型,支持 `true/false` 值
- 支持配置层级 merge (Local > Global > System)
- 默认值: `false` (使用固定大小分片)

### 完整配置示例

```toml
[fragment]
threshold = "1GB"          # 文件大小阈值,超过此值才进行分片 (默认 1GB,最小 1MiB)
size = "4m"                # 目标分片大小 (默认 1GB,最小 1MiB)
enable_cdc = true          # 启用 CDC 分片
```

**注意**: `threshold` 和 `size` 的最小有效值为 1MiB,低于此值将使用默认值。

### 配置层级

Zeta 的配置系统有三个层级 (优先级从低到高):

1. **System config** (`<prefix>/etc/zeta.toml`) - 系统级配置
2. **Global config** (`~/.zeta.toml`) - 用户全局配置
3. **Local config** (`.zeta/zeta.toml`) - 仓库本地配置 **(最高优先级)**

**Merge 语义**: 高优先级配置覆盖低优先级配置

```go
// Boolean.Merge() 实现
func (b *Boolean) Merge(other *Boolean) {
    // If other has a definite value, it should override b (higher priority)
    if other.val != BOOLEAN_UNSET {
        b.val = other.val
    }
}
```

---

## 五、调用链

### 入口函数

```go
// HashTo 是分片的主入口,根据文件大小和配置决定分片策略
func (r *Repository) HashTo(ctx context.Context, reader io.Reader, size int64) (oid plumbing.Hash, fragments bool, err error)
```

决策逻辑:
1. `size < Fragment.Threshold()` → 作为单个 blob 存储,不分片
2. `Fragment.EnableCDC.True()` → 调用 `writeCDCFragments` (CDC 分片)
3. 否则 → 调用 `writeFixedFragments` (固定大小分片)

### 实现文件

| 文件 | 说明 |
|------|------|
| `pkg/zeta/cdc.go` | FastCDC 分片器核心实现 (`Chunker`、`NewChunker`、`Walk`、`Split`) |
| `pkg/zeta/safetensors.go` | SafeTensors 格式解析器 (未来优化) |
| `pkg/zeta/objects.go` | `HashTo`、`writeCDCFragments`、`writeFixedFragments` |
| `modules/zeta/config/config.go` | Fragment 配置项定义 (`Fragment` struct) |
| `modules/zeta/config/type.go` | Boolean 类型实现 |

---

## 六、常见问题

### Q1: CDC 会影响读取性能吗?

**A**: 不会。读取时只根据 `Fragments.Entries` 中的偏移和大小读取,分片策略对读取透明。

### Q2: 已有仓库可以使用 CDC 吗?

**A**: 可以! CDC 只影响**新上传的文件**。已有文件保持原有分片方式,两种方式可以共存。

### Q3: CDC 分片大小不固定,如何优化存储?

**A**: CDC 分片大小在 `[minSize, maxSize]` 范围内波动,平均大小接近 `targetSize`。实际测试表明存储开销与固定分片相当。

### Q4: 为什么不能实现真正的 O(1) 空间复杂度?

**A**: CDC 的本质决定了它需要缓冲:
- CDC 需要读取字节 → 计算 hash → 检测边界
- 检测到边界后,才能哈希分片
- 但流已经读过去了,无法"回退"

**工业标准**: restic, borg, rsync 都使用 rolling buffer

### Q5: 默认的 targetSize 是多少?

**A**: 默认 targetSize 为 1GB (`FragmentSize` 常量)。对于 AI 模型场景,建议配置为更小的值如 `4m`,以获得更好的去重效果和更快的增量同步。

---

## 七、技术参考

1. **FastCDC 算法**: Xia, W., et al. "FastCDC: A Fast and Efficient Content-Defined Chunking Approach for Data Deduplication." USENIX ATC 2016
2. **Gear Hash**: 比传统 Rabin Fingerprint 快 2-3 倍
3. **CDC 原理**: "Content-Defined Chunking" (joshleeb.com)
4. **SafeTensors 格式**: https://huggingface.co/docs/safetensors
5. **工业实现参考**: restic, borg, rsync

---

**文档版本**: v3.0
**最后更新**: 2026-06-03
**维护者**: Zeta Team
