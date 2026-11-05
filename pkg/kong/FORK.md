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

我们定期从官方版同步新功能和修复。

### 2026-11-05 全量同步（上游 `258b13a` → HEAD `af6c651`）

本次采用三方合并（base=上游 `258b13a`，ours=魔改版，theirs=上游 HEAD），全量吸收上游 2026-02-07 至 2026-10-30 的约 40 个提交，并保留全部魔改功能。主要变更：

**新增功能**

| 功能 | 提交 | PR/Issue | 说明 |
|------|------|----------|------|
| `BindFor[T]` 泛型绑定 | `a2df5f5` | #590 | 替代 `BindTo(v, (*T)(nil))`，多数场景可自动推断类型 |
| last-wins 标志组 | `a60008c` | #653 | `lastwins:"group"` 组内后出现的取值覆盖先前值 |
| negatable bool mapper | `4e565e8` | #618 | 取反提升到令牌层（`--no-flag=false` 反转后再 Decode），自定义 bool mapper 拥有字段表示权；`flipBoolValue` 已删除 |
| 位置参数 `env` 标签 | `974c7fb` | #601 | `arg:"" env:"FOO"` 可从环境变量取默认值 |
| `envprefix` 传播 | `b778e12` | #635 | 子命令继承父级 `envprefix:` |
| 根节点 `Help()` | `14c97a2` | #593 | 根命令可实现 `HelpProvider` 提供帮助详情 |
| `NoAppDescFormat` | `bf3d068` | #597 | 跳过应用描述的换行/段落重排等格式化 |
| path 空默认值 | `db42183` | #592 | 显式 `--path=` 可清空带默认值的 path |
| Run 无需子命令 | `9d8d9f7` | #614 | 带 `Run()` 的节点可不带子命令直接执行 |

**重要修复**

| 修复 | 提交/PR | 说明 |
|------|---------|------|
| 默认命令拒绝兄弟命令标志 | `af6c651` / #679 | |
| 插值空回退 `${var=}` | `24be2f9` | 空默认值不再报 undefined variable |
| unexported 嵌入字段 hooks | `27d969c` / #620 | `getMethods` 重写为 `walkEmbedded` + `getExplicitMethod`（`runtime.FuncForPC` 识别编译器包装方法） |
| 重复嵌入回调 / 嵌入 `Validate()` | `ff16446` #616、`59826d1` #607 | |
| 多项 panic 修复 | #621 #623 #625 #626 #627 #651 | float 自整型、`set` 无值、filecontent 非 `[]byte`、counter `--flag=N`、map 键值类型、provider 值类型错误 |
| 配置覆盖无效默认值 | `49b8029` / #633 | |
| env 冲突 / env-only AfterApply / DefaultEnvars 后缀 | `c2c0229` #619、`7ee1766` #600、`0678fd3` #638 | |
| 单字符别名泄漏 / 视为短标志 | `33d3305` #637、`b03affc3` #605 | |

**冲突解决原则**：上游结构性重写（`getMethods`/`walkEmbedded`、令牌级取反、插值正则分组）按上游实现，fork 现代化写法（`Fields()` 迭代器、`errors.AsType`、`%w` 包装等）叠加其上。

**已知行为变化与使用约束**（对抗审查确认，当前 zeta 均未命中，仅作迁移备忘）：

- `negatable:` 仅适用于 Decode 时会消费 FlagValueToken 的 mapper（plain bool 均可）。类似 `VersionFlag` 这种不消费令牌的自定义 bool mapper 若加 negatable，将报 `unexpected flag argument` 错误而非翻转（上游 #618 令牌级取反的固有限制）。
- path 型显式空串（`--path=`）：结果为空字符串（上游 #592 有意设计，提示用 "."）；旧版为展开后的当前目录。zeta 的 `--cwd=`、`--config=` 属边缘用法。
- 带 `Run()` 且有子命令的父节点：缺子命令名不再报错，而是静默执行父 Run（上游 #614）。zeta 当前无此结构；新增此类结构时需自行校验。
- 上游 mapper.go 的 fileMapper 有 `HasDefault && Set` 早返回段（`theirs mapper.go:639`），本 fork 自基线前即缺失且 zeta 无 `type:"file"` 用法，未吸收；未来引入 os.File flag 时注意补齐。

**附带修复**（非 kong 代码）：

- `utils/cli/command_test.go`：`430d2b0d` 曾把 `cmd:"" name:"co"` 误改为 `cmd:"co"`（kong 语义中 `cmd` 标签值不参与命名，命令名取字段名），导致 `TestCheckout` 预存失败；本次恢复 `name:` 写法后测试通过。注意官方 CLI（`cmd/zeta/main.go`）一直使用 `cmd:"checkout" aliases:"co"`，不受影响。
- `pkg/kong/levenshtein.go`：清理一处复制粘贴损坏的注释。

### 2026-03-18 选择性移植

| 功能 | 提交/版本 | PR/Issue | 说明 |
|------|----------|----------|------|
| `WithHyphenPrefixedParameters` | `9bc3bf9` (v1.11.0) | #478, #315 | 允许参数值以 `-` 开头，如 `--number -10` |
| `Signature` 接口 | `95675de` | #581 | 类型可实现 `Signature() string` 提供默认 tag |
| `ValueFormatter` 暴露 | `d8de683` (v1.13.0) | #563 | `HelpOptions.ValueFormatter` 暴露给自定义 HelpPrinter |
| 变量插值改进 | `a62e6a4` | #555 | vars 值中可引用其他变量，如 `"default": "${config_file}/default"` |
| DynamicCommand 不强制 Run | `efa3691` (v1.12.1) | - | 移除动态命令必须有 `Run()` 方法的限制 |

### 魔改版已有的现代语法

魔改版已采用现代 Go 语法（zeta 工具链 go 1.27）：

- `reflect.Pointer` (Go 1.18+) - 替代 `reflect.Ptr`
- `errors.AsType[T]()` - 泛型错误处理
- `strings.SplitSeq()` (Go 1.24+) - 迭代器方法
- `for i := range n` / `for v := range value.Len()` - range-over-int 与迭代器
- `reflect.Value.Fields()` / `Methods()` - 结构字段与方法迭代器（Go 1.27）
- `reflect.TypeFor[T]()` - 替代 `reflect.TypeOf((*T)(nil)).Elem()`
- `maps.Copy` / `slices.Clone` / `slices.Contains` - 标准库集合操作
- `fmt.Errorf` 的 `%w` 包装 - 替代 `%s`/`%v`
- `max()` 函数 - 内置最大值函数
- `go/doc/comment.Printer` - 替代废弃的 `doc.ToText`
- `strings.Builder` - 字符串拼接
- `golang.org/x/text/cases.Title` - 替代废弃的 `strings.Title`

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

1. 克隆官方仓库到 `/tmp/kong`（保持最新）
2. 定位基线：找到上次同步时对应的上游提交（可比对本节各次同步记录的区间）
3. 对上游 `基线..HEAD` 区间内变更的每个非测试文件做三方合并：
   ```sh
   git show 基线:$f > base && git show HEAD:$f > theirs
   cp pkg/kong/$f ours
   git merge-file -L fork -L base -L upstream-HEAD -p ours base theirs > merged
   ```
4. 手工解决冲突：上游结构性重写按上游实现，fork 的 `W()` i18n、`PassthroughProvider`、`SummaryProvider`、安全检查与现代化语法必须保留；注意检查 `merged` 中自动合并区域是否引用了未定义的新符号（如本次的 `parseBoolToken` 定义在 mapper.go）
5. 落盘后运行：`go build ./... && go vet ./pkg/kong/ && go test ./pkg/kong/... ./utils/cli/...`
6. 验证命令行：`go run ./cmd/zeta/ --help` 与 `go run ./cmd/zeta/ co --help`
7. 更新本文件的同步记录（区间、功能、修复、附带修复）

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