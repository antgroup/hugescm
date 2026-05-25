# Changelog

## 2026-05-25 - 优化与清理

### 🚀 性能优化
- **字面量快速路径**：添加 `isLiteral` 优化标志
  - 字面量匹配性能提升 **20-25 倍**（59ns → 2.9ns）
  - 内存分配降至 **0**（48-96B → 0B）
  - 自动检测纯字面量模式，跳过复杂匹配逻辑

### ✅ 功能验证
- 验证所有功能正确性（19 个测试套件全部通过）
- 确认 Unicode 完全支持（文件名、路径）
- 确认 POSIX 字符类为 ASCII-only（与 Git 一致）

### 📝 文档完善
- 创建完整的 README.md
- 说明 Unicode 支持情况
- 添加使用示例和性能数据
- 说明与其他包的关系

### 🧹 代码清理
- 删除临时文档文件
- 整理目录结构
- 保持代码简洁

### 🗑️ 依赖清理
- wildmatch 包已删除（无外部引用）
- pathmatch 成为推荐的路径匹配方案

## 文件结构

```
modules/pathmatch/
├── README.md                    # 完整文档
├── pathmatch.go                 # 核心实现（已优化）
├── pathmatch_test.go            # 完整测试套件
├── pathmatch_fix_test.go        # 新增测试用例
├── pathmatch_casefold.go        # 大小写选项（macOS/Windows）
└── pathmatch_nocasefold.go      # 大小写选项（Linux）
```

## 性能数据

| 场景 | 优化前 | 优化后 | 提升 |
|------|--------|--------|------|
| 字面量匹配 | 59.54 ns/op | 2.886 ns/op | 20.6x |
| 分段路径匹配 | 73.08 ns/op | 2.887 ns/op | 25.3x |
| 内存分配 | 48-96 B/op | 0 B/op | 零分配 |

## 使用建议

### 推荐用于
- ✅ Git 分支/tag 匹配
- ✅ 文件路径过滤
- ✅ pathspec 模式匹配
- ✅ 高频字面量匹配场景

### 不推荐用于
- ❌ gitignore 匹配（使用 `plumbing/format/ignore`）
- ❌ 需要 Contents/Basename 语义的场景
