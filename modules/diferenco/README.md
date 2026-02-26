# Diferenco - Advanced Diff Algorithms / Diferenco - 高级 Diff 算法库

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

**Diferenco** is a comprehensive diff and merge library for Go that provides multiple algorithms for computing differences between sequences. It supports text, rune-level, and word-level diffing, along with three-way merge capabilities.

**Diferenco** 是一个功能全面的 Go 语言 diff 和合并库，提供多种算法来计算序列之间的差异。它支持文本、字符级和单词级 diff，以及三路合并功能。

## Features / 功能特性

- **Multiple Diff Algorithms / 多种 Diff 算法**
  - Myers - Classic O(ND) algorithm / 经典 O(ND) 算法
  - Histogram - Optimized for small files / 优化的小文件算法
  - ONP - Efficient for large files / 高效的大文件算法
  - Patience - Best for code structure / 最适合代码结构
  - Minimal - Simple and clean / 简单清晰

- **Multi-level Diffing / 多级 Diff**
  - Line-level diff / 行级 diff
  - Rune-level diff (character-based) / 字符级 diff
  - Word-level diff / 单词级 diff

- **Advanced Features / 高级功能**
  - Three-way merge (diff3) / 三路合并
  - Unified diff output / Unified diff 输出
  - Multiple conflict styles / 多种冲突样式
  - Context cancellation support / 支持上下文取消
  - Character set detection / 字符集检测

- **Performance Optimized / 性能优化**
  - Common prefix/suffix optimization / 公共前缀/后缀优化
  - Memory efficient implementations / 内存高效的实现
  - Benchmark-driven development / 基准测试驱动的开发

## Installation / 安装

```bash
go get code.alipay.com/zeta/zeta/modules/diferenco
```

## Quick Start / 快速开始

### Basic Line Diff / 基本行级 Diff

```go
package main

import (
    "context"
    "fmt"
    "code.alipay.com/zeta/zeta/modules/diferenco"
)

func main() {
    ctx := context.Background()

    before := []string{
        "Hello, World!",
        "This is line 2",
        "This is line 3",
    }

    after := []string{
        "Hello, World!",
        "This is modified line 2",
        "This is line 3",
        "This is new line 4",
    }

    // Compute diff using Myers algorithm / 使用 Myers 算法计算 diff
    changes, err := diferenco.MyersDiff(ctx, before, after)
    if err != nil {
        panic(err)
    }

    // Print changes / 打印变更
    for _, change := range changes {
        if change.Del > 0 {
            fmt.Printf("Deleted %d lines at position %d\n", change.Del, change.P1)
        }
        if change.Ins > 0 {
            fmt.Printf("Inserted %d lines at position %d\n", change.Ins, change.P2)
        }
    }
}
```

### Unified Diff Output / Unified Diff 输出

```go
opts := &diferenco.Options{
    From: &diferenco.File{
        Name: "old.txt",
        Hash: "abc123",
        Mode: 0644,
    },
    To: &diferenco.File{
        Name: "new.txt",
        Hash: "def456",
        Mode: 0644,
    },
    S1: "old file content",
    S2: "new file content",
    A:  diferenco.Histogram, // Use histogram algorithm / 使用 histogram 算法
}

unified, err := diferenco.DoUnified(ctx, opts)
if err != nil {
    panic(err)
}

fmt.Println(unified.String())
// Output:
// --- old.txt
// +++ new.txt
// @@ -1,2 +1,2 @@
// -old file
// +new file
//  content
```

### Character-level Diff / 字符级 Diff

```go
ctx := context.Background()
a := "The quick brown fox jumps over the lazy dog"
b := "The quick brown dog leaps over the lazy cat"

diffs, err := diferenco.DiffRunes(ctx, a, b, diferenco.Histogram)
if err != nil {
    panic(err)
}

for _, diff := range diffs {
    switch diff.Type {
    case diferenco.Equal:
        fmt.Print(diff.Text)
    case diferenco.Insert:
        fmt.Printf("\x1b[32m%s\x1b[0m", diff.Text) // Green / 绿色
    case diferenco.Delete:
        fmt.Printf("\x1b[31m%s\x1b[0m", diff.Text) // Red / 红色
    }
}
```

### Three-way Merge / 三路合并

```go
opts := &diferenco.MergeOptions{
    TextO: "Base content",        // Original / 原始内容
    TextA: "Branch A content",    // Your changes / 你的变更
    TextB: "Branch B content",    // Their changes / 他们的变更
    LabelO: "base",
    LabelA: "yours",
    LabelB: "theirs",
    A:      diferenco.Histogram,
}

result, hasConflicts, err := diferenco.Merge(ctx, opts)
if err != nil {
    panic(err)
}

if hasConflicts {
    fmt.Println("⚠️  Merge conflicts detected!")
} else {
    fmt.Println("✅ Merge successful!")
}

fmt.Println(result)
```

## Algorithm Selection / 算法选择

### Automatic Selection / 自动选择

```go
// Let the library choose the best algorithm / 让库选择最佳算法
changes, err := diferenco.diffInternal(ctx, before, after, diferenco.Unspecified)
```

The library automatically selects:
- **Histogram** for files < 5000 lines / 小于 5000 行的文件
- **ONP** for files >= 5000 lines / 大于等于 5000 行的文件

库自动选择：
- **Histogram** 用于小于 5000 行的文件
- **ONP** 用于大于等于 5000 行的文件

### Manual Selection / 手动选择

```go
// Explicitly specify algorithm / 明确指定算法
changes, err := diferenco.MyersDiff(ctx, before, after)
changes, err := diferenco.HistogramDiff(ctx, before, after)
changes, err := diferenco.OnpDiff(ctx, before, after)
changes, err := diferenco.PatienceDiff(ctx, before, after)
changes, err := diferenco.MinimalDiff(ctx, before, after)
```

### Algorithm Comparison / 算法对比

| Algorithm / 算法 | Speed / 速度 | Accuracy / 准确性 | Best For / 最适合 |
|-----------------|-------------|------------------|------------------|
| Myers / Myers | Medium / 中等 | High / 高 | General use / 通用 |
| Histogram / Histogram | Fast / 快 | High / 高 | Small files / 小文件 |
| ONP / ONP | Fast (few changes) / 快（少变更） | Medium / 中等 | Large files / 大文件 |
| Patience / Patience | Variable / 可变 | High for code / 代码中高 | Code files / 代码文件 |
| Minimal / Minimal | Medium / 中等 | High / 高 | Simple use / 简单使用 |

See [ALGORITHMS.md](ALGORITHMS.md) for detailed algorithm comparison.
详见 [ALGORITHMS.md](ALGORITHMS.md) 获取详细的算法对比。

## Advanced Usage / 高级用法

### Custom Word Splitting / 自定义单词分割

```go
customSplit := func(text string) []string {
    // Your custom word splitting logic / 你的自定义单词分割逻辑
    return strings.Fields(text)
}

diffs, err := diferenco.DiffWords(ctx, a, b, diferenco.Histogram, customSplit)
```

### Conflict Styles / 冲突样式

```go
// Default conflict style / 默认冲突样式
opts.Style = diferenco.STYLE_DEFAULT
// Output:
// <<<<<<< yours
// your change
// =======
// their change
// >>>>>>> theirs

// Diff3 style (includes base) / Diff3 样式（包含基础）
opts.Style = diferenco.STYLE_DIFF3
// Output:
// <<<<<<< yours
// your change
// ||||||| base
// base content
// =======
// their change
// >>>>>>> theirs

// Zealous diff3 style / Zealous diff3 样式
opts.Style = diferenco.STYLE_ZEALOUS_DIFF3
```

### File Statistics / 文件统计

```go
stats, err := diferenco.Stat(ctx, opts)
if err != nil {
    panic(err)
}

fmt.Printf("Additions: %d\n", stats.Addition)
fmt.Printf("Deletions: %d\n", stats.Deletion)
fmt.Printf("Hunks: %d\n", stats.Hunks)
```

### Context Cancellation / 上下文取消

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

changes, err := diferenco.MyersDiff(ctx, largeBefore, largeAfter)
if err == context.DeadlineExceeded {
    fmt.Println("Diff operation timed out")
} else if err != nil {
    panic(err)
}
```

## Performance / 性能

### Benchmarks / 基准测试

Run benchmarks to see performance with your data:
运行基准测试查看你的数据性能：

```bash
cd modules/diferenco
go test -bench=. -benchmem
```

Example results (your results may vary):
示例结果（你的结果可能不同）：

```
BenchmarkMyersAlgorithm/small_10pct_change-8           10000    123456 ns/op    34567 B/op    123 allocs/op
BenchmarkHistogramAlgorithm/small_10pct_change-8        20000     67890 ns/op    23456 B/op     89 allocs/op
BenchmarkONPAlgorithm/small_10pct_change-8             15000     98765 ns/op    45678 B/op    156 allocs/op
```

### Performance Tips / 性能建议

1. **Choose the right algorithm / 选择合适的算法**
   - Use Histogram for small files (< 5000 lines) / 小文件（< 5000 行）使用 Histogram
   - Use ONP for large files with few changes / 变化少的大文件使用 ONP
   - Use Patience for code with reordering / 重新排序的代码使用 Patience

2. **Pre-process when possible / 尽可能预处理**
   - Remove trailing whitespace / 移除尾随空格
   - Normalize line endings / 标准化行尾
   - Filter out comments if appropriate / 如果合适，过滤掉注释

3. **Use context with timeout / 使用带超时的上下文**
   - Prevent long-running operations / 防止长时间运行的操作
   - Handle cancellation gracefully / 优雅地处理取消

## Testing / 测试

### Run Tests / 运行测试

```bash
# Run all tests / 运行所有测试
go test ./...

# Run specific test / 运行特定测试
go test -v -run TestEmptyInputs

# Run tests with race detector / 使用竞态检测器运行测试
go test -race ./...

# Run tests with coverage / 运行测试并生成覆盖率
go test -cover ./...
```

### Run Benchmarks / 运行基准测试

```bash
# Run all benchmarks / 运行所有基准测试
go test -bench=. -benchmem

# Run specific benchmark / 运行特定基准测试
go test -bench=BenchmarkMyersAlgorithm -benchmem

# Compare benchmark results / 比较基准测试结果
go test -bench=. -benchmem | tee before.txt
# Make changes...
go test -bench=. -benchmem | tee after.txt
benchcmp before.txt after.txt
```

## Project Structure / 项目结构

```
modules/diferenco/
├── README.md                 # This file / 本文件
├── ALGORITHMS.md             # Algorithm documentation / 算法文档
├── diferenco.go              # Core functionality / 核心功能
├── myers.go                  # Myers algorithm / Myers 算法
├── histogram.go              # Histogram algorithm / Histogram 算法
├── onp.go                    # ONP algorithm / ONP 算法
├── patience.go               # Patience algorithm / Patience 算法
├── minimal.go                # Minimal algorithm / Minimal 算法
├── merge.go                  # Three-way merge / 三路合并
├── text.go                   # Text processing / 文本处理
├── sink.go                   # Line processing / 行处理
├── unified.go                # Unified diff output / Unified diff 输出
├── unified_encoder.go        # Output encoding / 输出编码
├── algorithms_test.go        # Unit tests / 单元测试
├── edge_cases_test.go        # Edge case tests / 边界测试
├── algorithms_bench_test.go  # Benchmarks / 基准测试
├── diferenco_test.go         # Integration tests / 集成测试
├── merge_test.go             # Merge tests / 合并测试
├── color/                    # Color output / 颜色输出
├── lcs/                      # LCS implementation / LCS 实现
└── testdata/                 # Test data / 测试数据
```

## Error Handling / 错误处理

All diff functions return errors that should be handled:

所有 diff 函数都返回应该处理的错误：

```go
changes, err := diferenco.MyersDiff(ctx, before, after)
if err != nil {
    switch {
    case errors.Is(err, diferenco.ErrUnsupportedAlgorithm):
        fmt.Println("Unsupported algorithm specified")
    case errors.Is(err, context.Canceled):
        fmt.Println("Operation canceled")
    case errors.Is(err, context.DeadlineExceeded):
        fmt.Println("Operation timed out")
    default:
        fmt.Printf("Error: %v\n", err)
    }
    return
}
```

## Contributing / 贡献

Contributions are welcome! Please:

欢迎贡献！请：

1. Fork the repository / Fork 仓库
2. Create a feature branch / 创建功能分支
3. Add tests for your changes / 为你的变更添加测试
4. Ensure all tests pass / 确保所有测试通过
5. Update documentation as needed / 根据需要更新文档
6. Submit a pull request / 提交 pull request

### Development Setup / 开发设置

```bash
# Clone the repository / 克隆仓库
git clone git@code.alipay.com:zeta/zeta.git
cd zeta/modules/diferenco

# Run tests / 运行测试
go test ./...

# Run linter / 运行代码检查
golangci-lint run

# Format code / 格式化代码
go fmt ./...
```

## License / 许可证

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件。

## Acknowledgments / 致谢

- Myers algorithm implementation inspired by Microsoft VSCode / Myers 算法实现受 Microsoft VSCode 启发
- Histogram algorithm based on imara-diff / Histogram 算法基于 imara-diff
- ONP algorithm from hattya/go.diff / ONP 算法来自 hattya/go.diff
- Patience algorithm based on Peter Evans' implementation / Patience 算法基于 Peter Evans 的实现
- Merge functionality inspired by diff3 implementation / 合并功能受 diff3 实现启发

## Support / 支持

For questions, issues, or contributions, please visit the project repository.

如有问题、疑问或贡献建议，请访问项目仓库。

## Changelog / 变更日志

### Version 1.0.0 / 版本 1.0.0
- Initial release / 初始版本
- Support for 5 diff algorithms / 支持 5 种 diff 算法
- Three-way merge support / 三路合并支持
- Comprehensive test coverage / 全面的测试覆盖
- Detailed documentation / 详细的文档

---

**Note:** This library is actively maintained. Check for updates regularly.
**注意：** 本库正在积极维护中。请定期检查更新。