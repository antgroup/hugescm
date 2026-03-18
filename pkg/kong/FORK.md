# Kong Fork 说明

本项目使用的是魔改版 [kong](https://github.com/alecthomas/kong)，基于官方版本进行了必要的定制化修改。

## 为什么需要魔改

官方 kong 在设计上存在一些不足，无法满足 zeta 作为 Git 兼容命令行工具的需求。以下是官方版缺失的关键功能：

### 1. PassthroughProvider 接口 - Git 风格命令参数

**问题**：官方版只支持 `Passthrough bool`，没有接口让命令自定义接收参数的方式。

**需求**：Git 命令经常使用 `--` 分隔符传递文件路径列表：

```bash
git checkout main -- file1.txt file2.txt
git diff HEAD -- path/to/file
git reset -- file1 file2
```

**实现**：`context.go:377-380`

```go
type PassthroughProvider interface {
    Passthrough([]string)
}
```

**使用示例**：

```go
// pkg/command/command_checkout.go
type Checkout struct { ... }

func (c *Checkout) Passthrough(paths []string) {
    c.passthroughArgs = append(c.passthroughArgs, paths...)
}
```

**无法绕过原因**：官方版没有提供任何机制让命令在 `--` 后接收动态参数列表。

---

### 2. 国际化支持 (W 函数) - 中文帮助文本

**问题**：官方版所有帮助文本硬编码英文，没有国际化机制。

**实现**：`hooks.go:34-44`

```go
var W = func(s string) string { return s }

func BindW(w func(s string) string) {
    W = w
}
```

**使用示例**：

```go
// 定义时包装需要翻译的文本
Help: W("Show context-sensitive help"),

// 启动时绑定翻译函数
kong.BindW(i18n.T)
```

**输出效果**：

```
用法：zeta checkout (co) [--branch|--tag] <url> [<destination>]
参数：
  [<args> ...]
标志：
  -h, --help    显示上下文相关的帮助
```

**替代方案对比**（antcode 方案）：

```go
// antcode 的拦截器方案 - 复杂且不精确
func KongHelpOptions() []kong.Option {
    return []kong.Option{
        kong.Help(helpPrinter),
        kong.WithBeforeResolve(func(ctx *kong.Context) error {
            translateApplication(ctx.Model)  // 运行时遍历
            return nil
        }),
    }
}

// 需要维护硬编码替换列表
replacements := []struct{from, to string}{
    {"Usage:", "用法："},
    {"Flags:", "选项："},
    // ... 可能误替换
}
```

| 特性 | antcode 方案 | zeta 方案 |
|------|-------------|----------|
| 代码量 | ~100 行 | ~10 行 |
| 精确性 | 可能误替换 | 精确控制 |
| 性能 | 运行时遍历 | 编译时确定 |
| 灵活性 | 硬编码列表 | 任意翻译函数 |

**无法绕过原因**：官方版没有提供任何国际化入口点。

---

### 3. SummaryProvider 接口 - Git 风格多用法显示

**问题**：官方版没有接口让命令自定义 Usage 行。

**需求**：Git 命令显示多种用法方式：

```
用法：git checkout [<options>] <branch>
  或：git checkout [<options>] [<branch>] -- <file>...
```

**实现**：`help.go:78-81`

```go
type SummaryProvider interface {
    Summary() string
}
```

**使用示例**：

```go
// pkg/command/command_checkout.go
func (c *Checkout) Summary() string {
    or := W("   或： ")
    return fmt.Sprintf(`
用法：zeta checkout (co) [--branch|--tag] <url> [<destination>]
%szeta checkout (co) <branch>
%szeta checkout (co) [<branch>] -- <file>...
`, or, or)
}
```

**降级影响**：

| 当前输出 | 降级后 |
|---------|--------|
| 显示 5 种用法方式 | 只显示 `zeta checkout <args>` |
| Git 风格友好体验 | 用户体验显著降低 |

---

### 4. 配置文件路径安全检查

**问题**：官方版 `LoadConfig` 直接打开用户提供的路径，没有安全检查。

**实现**：`kong.go:500-507`

```go
// Security: Check original path for absolute path to prevent unauthorized access
if filepath.IsAbs(path) {
    return nil, fmt.Errorf("absolute path not allowed for config file: %s", path)
}
// Security: Check original path for path traversal attempts
if strings.Contains(path, "..") {
    return nil, fmt.Errorf("path with '..' not allowed for config file: %s", path)
}
```

**防御的攻击**：

```bash
# 目录遍历攻击
zeta --config ../../etc/passwd

# 绝对路径访问
zeta --config /etc/shadow
```

---

## 从官方版同步的功能

我们定期从官方版同步新功能和修复。以下是本次（2026-03-18）从官方版移植的功能：

| 功能 | 提交/版本 | PR/Issue | 说明 |
|------|----------|----------|------|
| `WithHyphenPrefixedParameters` | `9bc3bf9` (v1.11.0) | #478, #315 | 允许参数值以 `-` 开头，如 `--number -10` |
| `Signature` 接口 | `95675de` | #581 | 类型可实现 `Signature() string` 提供默认 tag |
| `ValueFormatter` 暴露 | `d8de683` (v1.13.0) | #563 | `HelpOptions.ValueFormatter` 暴露给自定义 HelpPrinter |
| 变量插值改进 | `a62e6a4` | #555 | vars 值中可引用其他变量，如 `"default": "${config_file}/default"` |
| DynamicCommand 不强制 Run | `efa3691` (v1.12.1) | - | 移除动态命令必须有 `Run()` 方法的限制 |

### 魔改版已有的现代语法

魔改版已采用官方版的现代 Go 语法：

- `reflect.Pointer` (Go 1.18+) - 替代 `reflect.Ptr`
- `errors.AsType[T]()` - 泛型错误处理
- `strings.SplitSeq()` (Go 1.24+) - 迭代器方法
- `max()` 函数 - 内置最大值函数
- `go/doc/comment.Printer` - 替代废弃的 `doc.ToText`

### 魔改版独立的优化

| 优化 | 说明 |
|------|------|
| `guesswidth.go` 简化 | 使用 `golang.org/x/term.GetSize()` 替代 `syscall` + `unsafe`，移除平台特定的 build tag |

**之前**：2 个文件，手动 syscall 实现

```go
// guesswidth_unix.go - 仅 Unix 平台
var dimensions [4]uint16
syscall.Syscall6(syscall.SYS_IOCTL, ...)

// guesswidth.go - 其他平台 fallback
func guessWidth(_ io.Writer) int { return 80 }
```

**之后**：1 个文件，使用标准库

```go
// guesswidth.go - 所有平台通用
import "golang.org/x/term"

func guessWidth(w io.Writer) int {
    if f, ok := w.(*os.File); ok {
        if width, _, err := term.GetSize(int(f.Fd())); err == nil && width > 0 {
            return width
        }
    }
    return 80
}
```

---

## 文件结构

```
pkg/kong/
├── context.go      # PassthroughProvider 接口
├── hooks.go        # W() 国际化函数
├── help.go         # SummaryProvider 接口
├── kong.go         # LoadConfig 安全检查
├── options.go      # WithHyphenPrefixedParameters
├── scanner.go      # allowHyphenated 支持
├── tag.go          # Signature 接口
├── guesswidth.go   # 终端宽度检测（使用 golang.org/x/term）
└── FORK.md         # 本文档
```

## 维护指南

### 同步官方更新

1. 克隆官方仓库到 `/tmp/kong`
2. 对比差异：`diff -r pkg/kong/ /tmp/kong/`
3. 谨慎合并，保留魔改功能
4. 运行测试：`go test ./pkg/kong/...`
5. 验证命令行：`go run ./cmd/zeta/ --help`

### 核心功能不可删除

- ❌ `PassthroughProvider` - Git 风格命令依赖
- ❌ `W()` / `BindW()` - 国际化依赖
- ⚠️ `SummaryProvider` - Git 风格帮助依赖
- ⚠️ 路径安全检查 - 安全性依赖

## 参考资料

- 官方仓库：https://github.com/alecthomas/kong
- 官方文档：https://github.com/alecthomas/kong#readme
- zeta 使用示例：
  - `pkg/command/command_checkout.go` - PassthroughProvider + SummaryProvider
  - `pkg/command/command_diff.go` - PassthroughProvider + SummaryProvider
  - `utils/cli/command_test.go` - PassthroughProvider 测试