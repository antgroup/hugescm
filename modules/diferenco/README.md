# Diferenco - Advanced Diff Algorithms

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

**Diferenco** is a comprehensive diff and merge library for Go that provides multiple algorithms for computing differences between sequences. It supports text, rune-level, and word-level diffing, along with three-way merge capabilities.

**Diferenco** 是一个全面的 Go 语言 diff 和 merge 库，提供多种算法来计算序列之间的差异。支持文本、字符级和词级 diff，以及三路合并功能。

## Features / 特性

- **Multiple Diff Algorithms / 多种 Diff 算法**
  - **Myers** - Classic O(ND) algorithm, good for general use / 经典 O(ND) 算法，适合通用场景
  - **Histogram** - Fast and accurate, optimized for small files / 快速准确，针对小文件优化
  - **ONP** - O(NP) algorithm, efficient for large files with few changes / O(NP) 算法，适合大文件少改动
  - **Patience** - Unique-line based, best for code with reordering / 唯一行算法，适合代码重排序
  - **Minimal** - Simple implementation for basic use cases / 简单实现，适合基础场景
  - **SuffixArray** - LCS-based, efficient for text and binary data / 基于 LCS，适合文本和二进制数据

- **Multi-level Diffing / 多级 Diff**
  - Line-level diff / 行级 diff
  - Rune-level diff (character-based) / 字符级 diff
  - Word-level diff / 词级 diff

- **Advanced Features / 高级特性**
  - Three-way merge (diff3) / 三路合并
  - Unified diff output / 统一 diff 输出
  - Multiple conflict styles / 多种冲突样式
  - Context cancellation support / 支持上下文取消
  - Character set detection / 字符集检测

## Installation / 安装

```bash
go get github.com/antgroup/hugescm/modules/diferenco
```

## Quick Start / 快速开始

### Basic Line Diff / 基本行级 Diff

```go
package main

import (
    "context"
    "fmt"
    "github.com/antgroup/hugescm/modules/diferenco"
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

## Algorithm Comparison / 算法对比

| Algorithm | Time Complexity | Space Complexity | Best For |
|-----------|----------------|------------------|----------|
| **Myers** | O(ND) | O(D) | General use, balanced performance / 通用场景，均衡性能 |
| **Histogram** | O(N log N) | O(N) | Small files, high accuracy / 小文件，高精度 |
| **ONP** | O(NP) | O(N) | Large files with few changes / 大文件少改动 |
| **Patience** | O(N log N) | O(N) | Code with reordering, unique lines / 代码重排序，唯一行 |
| **Minimal** | O(N²) | O(N) | Simple use cases / 简单场景 |
| **SuffixArray** | O((N+M) log N) | O(N) | Text and binary data, LCS / 文本和二进制，LCS |

> N = total length, D = edit distance, P = number of changes / N=总长度，D=编辑距离，P=改动数

## Algorithm Details / 算法详解

### Myers Algorithm / Myers 算法

**English:**
The Myers algorithm, developed by Eugene Myers in 1986, is the classic diff algorithm used by Git. It finds the **shortest edit script (SES)** between two sequences.

**Core Idea / 核心思想:**
- Build an **edit graph** where each point (x,y) represents matching sequence1[0..x] with sequence2[0..y]
- Find the **shortest path** from (0,0) to (N,M)
- Diagonal moves (↘) are "free" (matching elements)
- Horizontal (→) = deletion, Vertical (↓) = insertion

**Implementation / 实现:**
```
         sequence1 (N)
         ────────────────
       │ . . . . . . . .
       │ . . . . . . . .
sequence│ . . . . ────────►
2 (M)   │ . . . .│  D  │
       │ . . . .│     │
       ▼ . . . .└─────┘
              (x,y) = endpoint
```

**Time Complexity / 时间复杂度:** O(ND) where D is the edit distance
- Worst case: O(N×M) when sequences are completely different
- Best case: O(N+M) when sequences are identical

**Pros / 优点:**
- Produces minimal edit scripts / 产生最小编辑脚本
- Well-tested, stable / 经过充分测试，稳定

**Cons / 缺点:**
- Can be slow for large files with many changes / 大文件多改动时可能较慢
- May produce unstable diffs with moved blocks / 移动块可能产生不稳定 diff

---

### Histogram Algorithm / Histogram 算法

**English:**
The Histogram algorithm is Git's default diff algorithm since 2010. It's based on the **patience diff** but uses **token frequency analysis** to find matches more intelligently.

**Core Idea / 核心思想:**
1. Build a **histogram** of token occurrences in both sequences
2. Find the **least frequent token** (most unique) to start matching
3. Extend matches forward and backward to find longest common subsequences
4. Recursively process unmatched regions

**Key Optimization / 关键优化:**
```go
// Prefer longest match first, then lowest occurrences for stability
// 优先最长匹配，长度相同时选择出现次数最少的（更稳定）
if length > s.lcs.length ||
    (length == s.lcs.length && occurrences < s.minOccurrences) {
    // select this match / 选择此匹配
}
```

**Time Complexity / 时间复杂度:** O(N log N) average case

**Pros / 优点:**
- Fast for most real-world cases / 大多数实际场景很快
- Produces clean, readable diffs / 产生清晰可读的 diff
- Avoids cross-matches / 避免交叉匹配

**Cons / 缺点:**
- Can degrade to O(N²) in worst case / 最坏情况可能退化为 O(N²)

---

### ONP Algorithm / ONP 算法

**English:**
The ONP (O(NP) Sequence Comparison) algorithm, developed by Sun Wu, Udi Manber, and Gene Myers, optimizes for the case where sequences have **few differences**.

**Core Idea / 核心思想:**
- Similar to Myers but optimizes for **small P** (number of changes)
- Uses a **greedy approach** with snake optimization
- Performance scales with **edit distance**, not total size

**Key Formula / 关键公式:**
```
Time = O((N+M) * D) where D is edit distance
     = O(NP) where P is min(N,M) for worst case
```

**Implementation / 实现:**
```go
// Uses furthest reaching path in each diagonal
// 使用每条对角线上最远可达路径
V[k] = furthest X value on diagonal k
```

**Pros / 优点:**
- Extremely fast for similar sequences / 相似序列极快
- Memory efficient / 内存高效

**Cons / 缺点:**
- Slow for completely different sequences / 完全不同序列较慢

---

### Patience Algorithm / Patience 算法

**English:**
The Patience algorithm, developed by Bram Cohen (creator of BitTorrent), focuses on finding **unique lines** as "anchors" and uses **LIS (Longest Increasing Subsequence)** to maintain order.

**Core Idea / 核心思想:**
1. Find lines that appear **exactly once** in both sequences (unique lines)
2. Match unique lines between sequences
3. Use **LIS** to find the longest sequence of matches that preserve order
4. Recursively diff the regions between anchors

**Why "Patience"? / 为什么叫 "Patience"?**
Named after the card game "Patience" (Solitaire), as the algorithm resembles sorting cards.

**Implementation / 实现:**
```go
// 1. Find unique lines / 找出唯一行
for i, e := range a {
    if count[e] == 1 {
        // unique element / 唯一元素
    }
}

// 2. LIS using binary search (O(N log N))
// 2. 使用二分查找的 LIS 算法 (O(N log N))
tails := make([]int, 0)
for _, p := range pairs {
    // binary search / 二分查找
    lo, hi := 0, len(tails)
    for lo < hi {
        mid := (lo + hi) / 2
        if pairs[tails[mid]].j < p.j {
            lo = mid + 1
        } else {
            hi = mid
        }
    }
}
```

**Time Complexity / 时间复杂度:**
- LIS: O(N log N) (optimized) / 优化后
- Overall: O(N log N) average case

**Pros / 优点:**
- Excellent for code with moved blocks / 适合移动块的代码
- Stable diffs, avoids jitter / 稳定的 diff，避免抖动
- Good for merge operations / 适合合并操作

**Cons / 缺点:**
- May miss non-unique matches / 可能错过非唯一匹配
- Requires enough unique lines / 需要足够多的唯一行

---

### Minimal Algorithm / Minimal 算法

**English:**
A simple implementation focused on correctness and ease of understanding. Uses a straightforward dynamic programming approach.

**Core Idea / 核心思想:**
- Build a **DP table** where `dp[i][j]` = LCS length for seq1[0..i] and seq2[0..j]
- Backtrack to find the actual changes

**Implementation / 实现:**
```go
// DP table / DP 表
for i := 1; i <= len(a); i++ {
    for j := 1; j <= len(b); j++ {
        if a[i-1] == b[j-1] {
            dp[i][j] = dp[i-1][j-1] + 1
        } else {
            dp[i][j] = max(dp[i-1][j], dp[i][j-1])
        }
    }
}
```

**Time Complexity / 时间复杂度:** O(N×M)

**Pros / 优点:**
- Simple, easy to understand / 简单易懂
- Good for learning / 适合学习

**Cons / 缺点:**
- Slow for large inputs / 大输入较慢
- O(N×M) memory / O(N×M) 内存

---

### SuffixArray Algorithm / SuffixArray 算法

**English:**
The SuffixArray algorithm uses a **suffix array** data structure to find the **longest common substring (LCS)** between sequences. This is different from LCS (Longest Common Subsequence).

**Core Idea / 核心思想:**
1. Build a **suffix array** for the first sequence
2. For each position in the second sequence, find the longest match in the suffix array
3. Recursively process unmatched regions

**Suffix Array / 后缀数组:**
```
Text: "banana"
Suffixes:          Sorted Suffixes:
banana   [0]       a        [5]
anana    [1]       ana      [3]
nana     [2]       anana    [1]
ana      [3]       banana   [0]
na       [4]       na       [4]
a        [5]       nana     [2]

Suffix Array: [5, 3, 1, 0, 4, 2]
```

**Implementation / 实现:**
```go
// Build suffix array using comparison sort
// 使用比较排序构建后缀数组
slices.SortFunc(indices, func(i, j int) int {
    return cmp.Compare(s[i], s[j])
})

// Find longest match using binary search
// 使用二分查找找最长匹配
slices.BinarySearchFunc(sa, target, func(idx int, target E) int {
    return cmp.Compare(data[idx], target)
})
```

**Time Complexity / 时间复杂度:** O((N+M) log N)
- Suffix array construction: O(N log N)
- Finding matches: O(M log N)

**Pros / 优点:**
- Efficient for text and binary data / 文本和二进制数据高效
- Good for finding repeated patterns / 适合查找重复模式
- Works with comparable types / 适用于可比较类型

**Cons / 缺点:**
- Requires `cmp.Ordered` types (int, string, etc.) / 需要 cmp.Ordered 类型
- Falls back to ONP for unsupported types / 不支持类型回退到 ONP

---

## Algorithm Selection Guide / 算法选择指南

### By Use Case / 按场景选择

| Use Case / 场景 | Recommended Algorithm / 推荐算法 |
|-----------------|-------------------------------|
| General purpose / 通用 | Myers, Histogram |
| Large files, few changes / 大文件少改动 | ONP |
| Code review, moved blocks / 代码审查，移动块 | Patience |
| Binary data / 二进制数据 | SuffixArray |
| Text with repeated patterns / 重复模式文本 | SuffixArray, Histogram |
| Small files / 小文件 | Histogram |
| Learning/Debugging / 学习/调试 | Minimal |

### By Performance / 按性能选择

```
Few Changes (D small) / 少改动:
  ONP > Histogram ≈ Patience > Myers > SuffixArray > Minimal

Many Changes (D large) / 多改动:
  Histogram > Patience > SuffixArray > Myers > ONP > Minimal

Large Files (N large) / 大文件:
  ONP > SuffixArray > Histogram > Patience > Myers > Minimal
```

## Advanced Usage / 高级用法

### Unified Diff Output / 统一 Diff 输出

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
    A:  diferenco.Histogram,
}

unified, err := diferenco.DoUnified(ctx, opts)
if err != nil {
    panic(err)
}

fmt.Println(unified.String())
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
    TextO: "Base content",        // Original / 原始
    TextA: "Branch A content",    // Your changes / 你的改动
    TextB: "Branch B content",    // Their changes / 他人的改动
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
    fmt.Println("Merge conflicts detected! / 检测到合并冲突!")
} else {
    fmt.Println("Merge successful! / 合并成功!")
}

fmt.Println(result)
```

### Context Cancellation / 上下文取消

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

changes, err := diferenco.MyersDiff(ctx, largeBefore, largeAfter)
if err == context.DeadlineExceeded {
    fmt.Println("Diff operation timed out / Diff 操作超时")
}
```

## Performance Tips / 性能建议

1. **Choose the right algorithm / 选择正确的算法**
   - Histogram for small files (< 5000 lines) / 小文件 (< 5000 行)
   - ONP for large files with few changes / 大文件少改动
   - Patience for code with reordering / 代码重排序
   - SuffixArray for text/binary data / 文本/二进制数据

2. **Pre-process when possible / 预处理**
   - Remove trailing whitespace / 移除尾部空白
   - Normalize line endings / 规范化行结束符
   - Filter out comments if appropriate / 适当过滤注释

3. **Use context with timeout / 使用带超时的上下文**
   - Prevent long-running operations / 防止长时间运行
   - Handle cancellation gracefully / 优雅处理取消

## Testing / 测试

```bash
# Run all tests / 运行所有测试
go test ./...

# Run with race detector / 运行竞态检测
go test -race ./...

# Run benchmarks / 运行基准测试
go test -bench=. -benchmem
```

## Project Structure / 项目结构

```
modules/diferenco/
├── diferenco.go          # Core functionality and public API / 核心功能和公共 API
├── myers.go              # Myers algorithm / Myers 算法
├── histogram.go          # Histogram algorithm / Histogram 算法
├── onp.go                # ONP algorithm / ONP 算法
├── patience.go           # Patience algorithm / Patience 算法
├── minimal.go            # Minimal algorithm / Minimal 算法
├── suffixarray.go        # SuffixArray algorithm / SuffixArray 算法
├── merge.go              # Three-way merge / 三路合并
├── text.go               # Text processing / 文本处理
├── unified.go            # Unified diff output / 统一 diff 输出
├── color/                # Color output utilities / 颜色输出工具
└── lcs/                  # LCS implementation / LCS 实现
```

## License / 许可证

MIT License - see [LICENSE](LICENSE) for details.
MIT 许可证 - 详见 [LICENSE](LICENSE)。

## Acknowledgments / 致谢

- Myers algorithm inspired by [Microsoft VSCode](https://github.com/microsoft/vscode)
- Histogram algorithm based on [imara-diff](https://github.com/pascalkuthe/imara-diff)
- ONP algorithm from [hattya/go.diff](https://github.com/hattya/go.diff)
- Patience algorithm based on [Peter Evans' implementation](https://github.com/peter-evans/patience-diff)
- SuffixArray algorithm inspired by [diff-match-patch](https://github.com/google/diff-match-patch)