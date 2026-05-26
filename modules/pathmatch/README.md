# pathmatch

轻量级的 glob 风格路径匹配库，专为 Git pathspec 场景优化（如 `git diff -- <path>`、分支/tag 匹配等）。

## 特性

- ✅ **高性能**：字面量匹配比原实现快 20-25 倍，零内存分配
- ✅ **完整的通配符支持**：`*`、`?`、`[...]`、`**`
- ✅ **Unicode 友好**：完全支持 Unicode 文件名和路径
- ✅ **Git 兼容**：与 Git wildmatch 行为一致
- ✅ **简洁高效**：避免复杂的 token 解析树，直接在路径段上匹配

## 快速开始

```go
import "github.com/antgroup/hugescm/modules/pathmatch"

// 创建模式
p := pathmatch.New("src/**/*.go")

// 匹配路径
if p.Match("src/pkg/main.go") {
    // 匹配成功
}

// 大小写不敏感匹配
p := pathmatch.New("Feature/*", pathmatch.CaseFold)
p.Match("feature/login") // true
```

## 支持的模式

### 基础通配符

- `*` - 匹配单个路径段内的任意字符序列
  ```go
  pathmatch.New("*.go").Match("main.go")        // true
  pathmatch.New("*.go").Match("pkg/main.go")    // false
  ```

- `?` - 匹配单个路径段内的单个字符
  ```go
  pathmatch.New("file?.txt").Match("file1.txt") // true
  pathmatch.New("file?.txt").Match("file12.txt") // false
  ```

- `[...]` - 字符类匹配
  ```go
  pathmatch.New("file[0-9].txt").Match("file5.txt")  // true
  pathmatch.New("file[!0-9].txt").Match("fileA.txt") // true
  ```

### 双星号（跨目录匹配）

- `**` - 作为独立段时，匹配零个或多个路径段
  ```go
  pathmatch.New("**/test").Match("test")           // true
  pathmatch.New("**/test").Match("a/b/c/test")     // true
  pathmatch.New("src/**/main.go").Match("src/main.go")       // true
  pathmatch.New("src/**/main.go").Match("src/pkg/main.go")   // true
  ```

- `**` - 嵌入在段中时，可以跨越 `/` 边界
  ```go
  pathmatch.New("foo**bar").Match("foobar")        // true
  pathmatch.New("foo**bar").Match("foo/baz/bar")   // true
  ```

### 字符类

- **字面量字符**：`[abc]` 匹配 a、b 或 c
- **范围**：`[a-z]` 匹配 a 到 z
- **否定**：`[!abc]` 或 `[^abc]` 匹配除 a、b、c 外的字符
- **POSIX 类**：`[:alpha:]`、`[:digit:]`、`[:alnum:]` 等（ASCII-only）

特殊规则：
- `]` 在首位时作为字面量：`[]abc]` 匹配 `]`、`a`、`b`、`c`
- `-` 在首位或末位时作为字面量：`[-abc]`、`[abc-]`

### 转义字符

使用 `\` 转义特殊字符：
```go
pathmatch.New(`foo\*bar`).Match("foo*bar")   // true
pathmatch.New(`foo\*bar`).Match("foobar")    // false
pathmatch.New(`\[test\]`).Match("[test]")    // true
```

## Unicode 支持

### ✅ 完全支持 Unicode 路径

```go
// 中文路径
pathmatch.New("文档/*.txt").Match("文档/测试.txt")     // true

// emoji
pathmatch.New("📁/*.go").Match("📁/main.go")          // true

// 日文
pathmatch.New("**/テスト").Match("a/b/テスト")        // true
```

### ⚠️ POSIX 字符类是 ASCII-only

POSIX 字符类（如 `[:alpha:]`、`[:digit:]`）仅匹配 ASCII 字符，与 Git 行为一致：

```go
pathmatch.New("[[:alpha:]]").Match("a")   // true
pathmatch.New("[[:alpha:]]").Match("中")  // false (不匹配 Unicode)
pathmatch.New("[[:digit:]]").Match("5")   // true
```

**原因**：
- Git ref 名称（分支、tag）通常是 ASCII
- 保持与 Git wildmatch.c 行为一致
- 性能优化（使用预计算查找表）

**替代方案**：使用 `*` 或 `?` 通配符匹配 Unicode 字符。

## 选项

### CaseFold - 大小写不敏感

```go
p := pathmatch.New("Feature/*", pathmatch.CaseFold)
p.Match("feature/login")  // true
p.Match("FEATURE/LOGIN")  // true
```

### SystemCase - 系统相关大小写

根据操作系统自动选择是否大小写敏感：
- macOS/Windows：大小写不敏感
- Linux：大小写敏感

```go
p := pathmatch.New("src/*.go", pathmatch.SystemCase)
```

## 性能

### 基准测试结果

在 Apple M4 Pro 上的性能数据：

| 场景 | 性能 | 内存分配 |
|------|------|----------|
| 字面量匹配 | 2.9 ns/op | 0 B/op |
| 通配符匹配 | 28 ns/op | 16 B/op |
| 双星号匹配 | 57 ns/op | 48 B/op |
| 复杂模式 | 115 ns/op | 96 B/op |

### 优化亮点

**字面量快速路径**：
- 自动检测纯字面量模式（无通配符）
- 直接字符串比较，跳过复杂匹配逻辑
- 性能提升 **20-25 倍**，零内存分配

```go
// 这些模式会使用快速路径
pathmatch.New("main")
pathmatch.New("src/pkg/main.go")
pathmatch.New("feature/login")
```

## 使用场景

### Git 分支匹配

```go
// 匹配所有 feature 分支
p := pathmatch.New("feature/*")
p.Match("feature/login")    // true
p.Match("feature/api")      // true
p.Match("main")             // false

// 匹配 release 分支
p := pathmatch.New("release-*")
p.Match("release-1.0")      // true
```

### Git tag 匹配

```go
// 匹配版本 tag
p := pathmatch.New("v[0-9].*")
p.Match("v1.0")             // true
p.Match("v2.3.1")           // true

// 匹配所有 v1.x 版本
p := pathmatch.New("v1.*")
p.Match("v1.0")             // true
p.Match("v2.0")             // false
```

### 文件路径匹配

```go
// 匹配所有 Go 文件
p := pathmatch.New("**/*.go")
p.Match("main.go")                    // true
p.Match("src/pkg/main.go")            // true
p.Match("a/b/c/d/file.go")            // true

// 匹配测试文件
p := pathmatch.New("**/*_test.go")
p.Match("pkg/util_test.go")           // true
```

## 与 Git 的兼容性

pathmatch 实现了 Git pathspec 的核心匹配语义：

- ✅ 通配符行为与 Git 一致
- ✅ `**` 语义与 Git 一致
- ✅ 字符类与 Git 一致
- ✅ 转义字符与 Git 一致
- ✅ 通过 Git 官方 wildmatch 测试套件

**不支持的 Git 特性**（不需要）：
- ❌ Basename 模式（pathspec 总是全路径匹配）
- ❌ Contents 模式（gitignore 特有，使用 `plumbing/format/ignore` 包）
- ❌ GitAttributes 模式

## 测试

```bash
# 运行所有测试
go test

# 运行基准测试
go test -bench=. -benchmem

# 运行特定测试
go test -run TestGitWildmatch -v
```

测试覆盖：
- ✅ Git 官方 wildmatch 测试套件
- ✅ 边界条件测试
- ✅ Unicode 测试
- ✅ 性能基准测试

## 实现细节

### 设计原则

1. **轻量级**：避免复杂的 token 解析树，直接在路径段上匹配
2. **高性能**：预计算优化标志，字面量快速路径
3. **正确性**：与 Git wildmatch 行为完全一致

### 架构

```
Pattern 结构：
  - segments: 路径段数组（按 '/' 分割）
  - isLiteral: 是否为纯字面量（优化标志）
  - literalPath: 字面量路径缓存
  - hasDoubleStar: 是否包含 **
  - caseFold: 是否大小写不敏感

匹配流程：
  1. 预处理：规范化路径，移除尾部 '/'
  2. 快速路径：字面量直接字符串比较
  3. 递归匹配：逐段匹配，处理 ** 回溯
```

## 与其他包的关系

项目中有两个独立的模式匹配实现：

1. **pathmatch**（本包）
   - 用途：路径/分支/tag 匹配
   - 特点：轻量级、高性能
   - 推荐用于：Git pathspec、分支匹配、文件路径过滤

2. **plumbing/format/ignore**
   - 用途：gitignore 专用匹配
   - 特点：支持 gitignore 特有语义（Contents、否定等）
   - 推荐用于：.gitignore 文件处理

## 许可证

与项目主许可证一致。

## 贡献

欢迎提交 issue 和 pull request！
