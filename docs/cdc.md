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
阶段 2 (minSize ~ normalSize): 使用 maskS (最高切割概率)
阶段 3 (normalSize ~ normalSize+window): 使用 maskN (标准切割概率)
阶段 4 (normalSize+window ~ maxSize): 使用 maskL (最低切割概率)
阶段 5 (maxSize+): 强制切割
```

**三 mask 策略**:
- `maskS = 2^(bits-2) - 1`: 最高切割概率 (快速跳过小分片)
- `maskN = 2^bits - 1`: 标准切割概率
- `maskL = 2^(bits+1) - 1`: 最低切割概率 (允许更大分片)

### 参数配置

**默认参数 (针对 AI 模型优化)**:

```go
targetSize = 4MB   // 目标分片大小
minSize    = 1MB   // 最小分片 (target / 4)
maxSize    = 32MB  // 最大分片 (target * 8)
```

**为什么选择 4MB?**

AI 模型文件的特点:
- 典型张量大小: 几 MB 到几百 MB
- Fine-tuning 更新: 通常是整个张量或较大区域
- Checkpoint 文件: 10GB - 100GB

**4MB 分片的优势**:

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
// 缓冲区大小 = maxChunkSize (通常 32MB)
chunkBuf := make([]byte, 0, c.maxSize)

// 检测到边界后
onChunk(offset, size int64, chunkReader io.Reader)
```

**内存占用**: O(maxChunkSize)
- 典型值: 32MB (maxSize = target * 8)
- 与文件大小无关,只与分片大小有关

**这是工业标准做法**:
- restic: 使用滚动缓冲区
- borg: 使用滚动缓冲区
- rsync: 使用滚动缓冲区

### Pipeline 设计

**单遍扫描,零临时文件**:

```go
func (r *Repository) hashToWithCDC(ctx context.Context, reader io.Reader, size int64) (oid plumbing.Hash, fragments bool, err error) {
    // 1. 计算完整文件哈希
    h := plumbing.NewHasher()
    teeReader := io.TeeReader(reader, h)

    // 2. 创建 CDC 分片器
    cdcChunker := NewCDCChunker(r.Fragment.Size())

    // 3. 单遍流式分片 + 哈希计算
    err = cdcChunker.ChunkStreaming(teeReader, size, func(offset, chunkSize int64, chunkReader io.Reader) error {
        chunkHash, _ := r.odb.HashTo(ctx, chunkReader, chunkSize)
        ff.Entries = append(ff.Entries, &object.Fragment{
            Index: chunkIndex,
            Hash:  chunkHash,
            Size:  uint64(chunkSize),
        })
        return nil
    })

    // 4. 保存 Fragments 对象
    ff.Origin = h.Sum()
    oid, _ = r.odb.WriteEncoded(ff)
    return
}
```

**优点**:
- 单 pass
- full hash + chunk hash 同时算
- 无临时文件

---

## 四、配置使用

### 启用 CDC

在 `.zeta/config` 文件中添加:

```toml
[fragment]
enable_cdc = true          # 启用 CDC 分片 (Boolean 类型,支持配置 merge)
```

**配置说明**:
- `enable_cdc` 是 `Boolean` 类型,支持 `true/false` 值
- 支持配置层级 merge (Local > Global > System)
- 默认值: `false` (使用固定大小分片)

### 配置层级

Zeta 的配置系统有三个层级 (优先级从低到高):

1. **System config** (`/etc/zeta/config`) - 系统级配置
2. **Global config** (`~/.zeta/config`) - 用户全局配置
3. **Local config** (`.zeta/config`) - 仓库本地配置 **(最高优先级)**

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

## 五、实现文件

| 文件 | 说明 |
|------|------|
| `pkg/zeta/cdc.go` | FastCDC 分片器核心实现 |
| `pkg/zeta/safetensors.go` | SafeTensors 格式解析器 (未来优化) |
| `pkg/zeta/objects.go` | `hashToWithCDC` 主入口函数 |
| `modules/zeta/config/config.go` | CDC 配置项定义 |
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

---

## 七、技术参考

1. **FastCDC 算法**: Xia, W., et al. "FastCDC: A Fast and Efficient Content-Defined Chunking Approach for Data Deduplication." USENIX ATC 2016
2. **Gear Hash**: 比传统 Rabin Fingerprint 快 2-3 倍
3. **CDC 原理**: "Content-Defined Chunking" (joshleeb.com)
4. **SafeTensors 格式**: https://huggingface.co/docs/safetensors
5. **工业实现参考**: restic, borg, rsync

---

**文档版本**: v2.0
**最后更新**: 2026-03-17
**维护者**: Zeta Team