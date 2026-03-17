# Zeta 文档中心

欢迎来到 Zeta 文档中心! Zeta 是面向 AI 场景的巨型存储库版本控制系统。

---

## 📚 文档索引

### 核心概念

| 文档 | 描述 |
|------|------|
| [protocol.md](protocol.md) | Zeta 协议规范 |
| [object-format.md](object-format.md) | 对象格式详解 |
| [pack-format.md](pack-format.md) | Pack 文件格式 |
| [config.md](config.md) | 配置文件说明 |

### CDC 分片功能 (新功能)

| 文档 | 描述 |
|------|------|
| [cdc.md](cdc.md) | CDC (Content-Defined Chunking) 实现原理、架构设计和配置 |
| [zeta.toml.example](zeta.toml.example) | CDC 配置示例 |

### 高级功能

| 文档 | 描述 |
|------|------|
| [sparse-checkout.md](sparse-checkout.md) | 稀疏检出策略 |
| [pull-strategy.md](pull-strategy.md) | 拉取策略 |
| [stash.md](stash.md) | 暂存功能 |
| [version-negotiation.md](version-negotiation.md) | 版本协商机制 |

---

## 🚀 快速开始

### 1. 启用 CDC 分片 (推荐用于 AI 模型文件)

在 `.zeta/zeta.toml` 文件中添加配置（或运行 `zeta config --add -T bool fragment.enable_cdc  true`）:

```toml
[fragment]
threshold = "1GB"      # 文件大小阈值,超过此大小才分片
size = "4GB"           # 目标分片大小 (推荐 4GB 用于 AI 模型)
enable_cdc = true      # 启用 CDC 分片
```

**配置文件位置**:
- **Local config**: `.zeta/zeta.toml` (仓库级,最高优先级)
- **Global config**: `~/.zeta.toml` (用户级)
- **System config**: `/etc/zeta.toml` (系统级)

### 2. CDC 分片的优势

CDC (Content-Defined Chunking) 相比固定大小分片,在以下场景有明显优势:

- **模型微调**: 只传输变化的权重,未修改的权重无需重复传输
- **插入数据**: 局部修改不会影响整个文件的分片边界
- **版本迭代**: 相同内容自动识别,避免重复存储

### 3. 配置说明

#### `[fragment]` 配置项

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `threshold` | Size | `1GB` | 文件大小阈值,小于此值不分片 |
| `size` | Size | `4GB` | 目标分片大小 (固定分片大小) |
| `enable_cdc` | Boolean | `false` | 启用 CDC 分片功能 (支持配置 merge) |

#### Size 格式支持

支持以下单位:
- `KB`, `MB`, `GB` (1000 进制)
- `KiB`, `MiB`, `GiB` (1024 进制)

示例:
```toml
threshold = "512MiB"
size = "4GB"
```

---

## 📖 核心概念

### CDC 实现原理

CDC (Content-Defined Chunking,内容定义分片) 是一种根据数据内容自动确定分片边界的算法。

**核心特性**:
- ✅ **边界稳定**: 插入/删除数据不会影响其他分片边界
- ✅ **高去重率**: 相同内容自动识别,避免重复存储
- ✅ **FastCDC 算法**: 三阶段归一化切割,工业级实现
- ✅ **流式处理**: Rolling buffer,内存占用可控 (O(maxChunkSize))

详细实现请参考: [cdc.md](cdc.md)

### 分片参数选择

Zeta 的默认参数针对 **AI 模型文件**进行了优化:

```
targetSize = 4MB
minSize    = 1MB   (target / 4)
maxSize    = 32MB  (target * 8)
```

**为什么选择 4MB?**

| 场景 | 1MB 分片 | 4MB 分片 | 优势 |
|------|---------|---------|------|
| 10GB 模型 | ~10000 fragments | ~2500 fragments | 减少 75% 元数据 |
| 去重效果 | 高 | 高 (相近) | 模型文件增量更新效果类似 |
| 传输协商 | 较慢 | 更快 | fragment 数量少,metadata 传输快 |
| CPU 开销 | 较高 | 较低 | hash 计算次数减少 |

**适用场景**:
- ✅ AI 模型文件 (checkpoint, weights, etc.)
- ✅ 大型二进制文件
- ✅ 游戏资源
- ⚠️ 小型文本文件 (建议保持默认配置)

### 配置层级和优先级

Zeta 的配置系统支持三层配置 (优先级从低到高):

1. **System config** (`/etc/zeta/config`) - 最低优先级
2. **Global config** (`~/.zeta/config`) - 中等优先级
3. **Local config** (`.zeta/config`) - **最高优先级**

配置会按优先级自动 merge,高优先级配置覆盖低优先级。

**Boolean.Merge() 实现**:

```go
// 高优先级配置生效
func (b *Boolean) Merge(other *Boolean) {
    if other.val != BOOLEAN_UNSET {
        b.val = other.val
    }
}
```

---

## 🔧 故障排查

### 常见问题

#### 1. CDC 没有生效?

检查配置文件是否正确:
```bash
# 查看当前配置
cat .zeta/config

# 确认 enable_cdc 设置
grep enable_cdc .zeta/config
```

#### 2. 分片大小不符合预期?

CDC 是**平均分片大小**,实际分片大小会在 `minSize` 到 `maxSize` 之间波动:
- `minSize` = target / 4 (例如 target=4MB 时,minSize=1MB)
- `maxSize` = target * 8 (例如 target=4MB 时,maxSize=32MB)

#### 3. 如何验证 CDC 效果?

```bash
# 存储文件后查看分片信息
zeta ls-tree -r HEAD

# 查看分片详情
zeta cat-file -p <hash>
```

---

## 📞 获取帮助

- **问题反馈**: 提交 Issue 到内部代码仓库
- **技术支持**: 联系 Zeta 团队
- **文档改进**: 提交 PR 完善文档

---

## 📝 文档更新

- 2026-03-17: 添加 CDC 分片功能文档
- 2025-08-20: 初始文档创建